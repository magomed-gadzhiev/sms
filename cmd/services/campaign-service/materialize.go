package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
	campaignrepo "github.com/smpp-server/smpp-server/internal/services/campaign/infrastructure/repository"
	smstpl "github.com/smpp-server/smpp-server/internal/shared/template"
)

// kafkaOutgoingMsg — формат сообщения для топика sms.outgoing (совпадает с queue.KafkaMessage).
type kafkaOutgoingMsg struct {
	ID          string     `json:"id"`
	MessageID   uuid.UUID  `json:"message_id"`
	Source      string     `json:"source"`
	Destination string     `json:"destination"`
	Text        string     `json:"text"`
	ClientID    *uuid.UUID `json:"client_id,omitempty"`
	Priority    int        `json:"priority"`
	RetryCount  int        `json:"retry_count"`
	MaxRetries  int        `json:"max_retries"`
	CreatedAt   time.Time  `json:"created_at"`
}

// startMaterializationWorker запускает горутину, которая каждые 5 секунд
// ищет кампании со статусом "materializing" и разворачивает их в сообщения.
func startMaterializationWorker(
	ctx context.Context,
	dbx *sqlx.DB,
	producer sarama.SyncProducer,
	topicOutgoing string,
	recipientRepo *campaignrepo.RecipientRepository,
	logger zerolog.Logger,
) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processMaterializingCampaigns(ctx, dbx, producer, topicOutgoing, recipientRepo, logger); err != nil {
					logger.Error().Err(err).Msg("ошибка материализации кампаний")
				}
			}
		}
	}()
}

func processMaterializingCampaigns(
	ctx context.Context,
	dbx *sqlx.DB,
	producer sarama.SyncProducer,
	topicOutgoing string,
	recipientRepo *campaignrepo.RecipientRepository,
	logger zerolog.Logger,
) error {
	type campRow struct {
		ID            string
		ClientID      string
		ContactListID string
		TemplateID    *string
		Source        string
	}

	rows, err := dbx.QueryContext(ctx,
		`SELECT id, client_id, contact_list_id, template_id, source
		 FROM campaigns WHERE status = 'materializing' LIMIT 5`)
	if err != nil {
		return fmt.Errorf("query materializing: %w", err)
	}
	defer rows.Close()

	var campaigns []campRow
	for rows.Next() {
		var c campRow
		if err := rows.Scan(&c.ID, &c.ClientID, &c.ContactListID, &c.TemplateID, &c.Source); err != nil {
			continue
		}
		campaigns = append(campaigns, c)
	}
	rows.Close()

	for _, c := range campaigns {
		if err := materializeCampaign(ctx, dbx, producer, topicOutgoing, recipientRepo, c.ID, c.ClientID, c.ContactListID, c.TemplateID, c.Source, logger); err != nil {
			logger.Error().Err(err).Str("campaign_id", c.ID).Msg("ошибка материализации кампании")
		}
	}
	return nil
}

func materializeCampaign(
	ctx context.Context,
	dbx *sqlx.DB,
	producer sarama.SyncProducer,
	topicOutgoing string,
	recipientRepo *campaignrepo.RecipientRepository,
	campaignID, clientID, contactListID string,
	templateID *string,
	source string,
	logger zerolog.Logger,
) error {
	// Atomically claim this campaign to avoid double-processing.
	result, err := dbx.ExecContext(ctx,
		`UPDATE campaigns SET status = 'running', updated_at = now()
		 WHERE id = $1 AND status = 'materializing'`, campaignID)
	if err != nil {
		return fmt.Errorf("claim campaign: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil // another instance already picked it up
	}

	// If recipients already exist (e.g. from a previous partial run) we're done.
	var recipientCount int
	if err := dbx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM campaign_recipients WHERE campaign_id = $1`, campaignID,
	).Scan(&recipientCount); err != nil {
		return err
	}
	if recipientCount > 0 {
		logger.Info().Str("campaign_id", campaignID).Msg("получатели уже существуют, кампания уже запущена")
		return nil
	}

	clientUUID, err := uuid.Parse(clientID)
	if err != nil {
		return fmt.Errorf("invalid client_id: %w", err)
	}
	campaignUUID, err := uuid.Parse(campaignID)
	if err != nil {
		return fmt.Errorf("invalid campaign_id: %w", err)
	}

	// Get template body (best-effort).
	var templateBody string
	if templateID != nil && *templateID != "" {
		_ = dbx.QueryRowContext(ctx, `SELECT body FROM templates WHERE id = $1`, *templateID).Scan(&templateBody)
	}

	// Fetch contacts.
	type contactRow struct {
		ID         string
		Phone      string
		Attributes []byte
	}
	cRows, err := dbx.QueryContext(ctx,
		`SELECT id, phone, attributes FROM contacts WHERE contact_list_id = $1 ORDER BY created_at`, contactListID)
	if err != nil {
		return fmt.Errorf("fetch contacts: %w", err)
	}
	defer cRows.Close()

	var contacts []contactRow
	for cRows.Next() {
		var cr contactRow
		if err := cRows.Scan(&cr.ID, &cr.Phone, &cr.Attributes); err != nil {
			continue
		}
		contacts = append(contacts, cr)
	}
	cRows.Close()

	if len(contacts) == 0 {
		_, err = dbx.ExecContext(ctx,
			`UPDATE campaigns SET status = 'completed', total_recipients = 0, updated_at = now() WHERE id = $1`, campaignID)
		return err
	}

	renderer := smstpl.NewRenderer()
	var recipients []domain.Recipient

	for _, contact := range contacts {
		contactUUID, _ := uuid.Parse(contact.ID)
		msgID := uuid.New()
		recipientID := uuid.New()
		now := time.Now()

		// Render template with contact attributes.
		text := templateBody
		if templateBody != "" {
			var bindings map[string]interface{}
			if jsonErr := json.Unmarshal(contact.Attributes, &bindings); jsonErr == nil {
				if rendered, renderErr := renderer.Render(templateBody, bindings); renderErr == nil {
					text = rendered
				}
			}
		}

		// Insert message into DB.
		_, err := dbx.ExecContext(ctx, `
			INSERT INTO messages (
				id, source, destination, text, encoding, data_coding, esm_class,
				protocol_id, priority_flag, replace_if_present, registered_delivery,
				service_type, source_addr_ton, source_addr_npi, dest_addr_ton, dest_addr_npi,
				status, client_id, retry_count, max_retries, created_at, updated_at
			) VALUES ($1,$2,$3,$4,'GSM7',0,0,0,0,0,1,'',0,0,0,0,'queued',$5,0,5,$6,$6)`,
			msgID, source, contact.Phone, text, clientUUID, now,
		)
		if err != nil {
			logger.Error().Err(err).Str("phone", contact.Phone).Msg("ошибка вставки сообщения")
			continue
		}

		// Publish to Kafka.
		if producer != nil {
			km := kafkaOutgoingMsg{
				ID: msgID.String(), MessageID: msgID,
				Source: source, Destination: contact.Phone, Text: text,
				ClientID: &clientUUID, MaxRetries: 5, CreatedAt: now,
			}
			data, _ := json.Marshal(km)
			if _, _, kafkaErr := producer.SendMessage(&sarama.ProducerMessage{
				Topic: topicOutgoing,
				Value: sarama.ByteEncoder(data),
			}); kafkaErr != nil {
				logger.Error().Err(kafkaErr).Str("message_id", msgID.String()).Msg("ошибка публикации в Kafka")
			}
		}

		recipients = append(recipients, domain.Recipient{
			ID:         recipientID,
			CampaignID: campaignUUID,
			ContactID:  contactUUID,
			Phone:      contact.Phone,
			Status:     domain.RecipientPending,
			MessageID:  &msgID,
		})
	}

	// Bulk insert recipients.
	if len(recipients) > 0 {
		if err := recipientRepo.BulkInsert(ctx, recipients); err != nil {
			return fmt.Errorf("bulk insert recipients: %w", err)
		}
	}

	// Update total_recipients count.
	_, err = dbx.ExecContext(ctx,
		`UPDATE campaigns SET total_recipients = $1, started_at = COALESCE(started_at, now()), updated_at = now() WHERE id = $2`,
		len(contacts), campaignID)
	if err != nil {
		return fmt.Errorf("update total_recipients: %w", err)
	}

	logger.Info().
		Str("campaign_id", campaignID).
		Int("contacts", len(contacts)).
		Int("queued", len(recipients)).
		Msg("кампания материализована и запущена")

	return nil
}
