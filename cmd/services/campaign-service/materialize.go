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
	TrafficType string     `json:"traffic_type,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// startMaterializationWorker запускает горутину, которая каждые 5 секунд
// ищет кампании со статусом "materializing" и разворачивает их в сообщения.
// Также запускает воркер для досылки сообщений у получателей без message_id.
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

	// Worker to resend messages for recipients with NULL message_id (partial materialization recovery)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processUnsentRecipients(ctx, dbx, producer, topicOutgoing, logger); err != nil {
					logger.Error().Err(err).Msg("ошибка досылки сообщений")
				}
			}
		}
	}()

	// Worker to re-publish messages that are stuck in 'queued' state
	// (Kafka publish failed during materialization).
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processStaleQueued(ctx, dbx, producer, topicOutgoing, 5*time.Minute, logger); err != nil {
					logger.Error().Err(err).Msg("ошибка переотправки stale queued сообщений")
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

	// Get template body and traffic_type (best-effort).
	var templateBody, templateTrafficType string
	if templateID != nil && *templateID != "" {
		_ = dbx.QueryRowContext(ctx, `SELECT body, COALESCE(traffic_type, 'transactional') FROM templates WHERE id = $1`, *templateID).Scan(&templateBody, &templateTrafficType)
	}
	if templateTrafficType == "" {
		templateTrafficType = "transactional"
	}

	// Fetch contacts, excluding those that have opted out.
	type contactRow struct {
		ID         string
		Phone      string
		Attributes []byte
	}
	cRows, err := dbx.QueryContext(ctx, `
		SELECT c.id, c.phone, c.attributes
		FROM contacts c
		WHERE c.contact_list_id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM opt_out_list o
		      WHERE o.client_id = $2::uuid AND o.phone = c.phone
		  )
		ORDER BY c.created_at`, contactListID, clientID)
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
				TrafficType: templateTrafficType,
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

// processUnsentRecipients находит получателей с message_id = NULL в running кампаниях
// и создаёт/отправляет для них сообщения. Это восстановление после частичной материализации
// (например, Kafka был недоступен при первой попытке).
func processUnsentRecipients(
	ctx context.Context,
	dbx *sqlx.DB,
	producer sarama.SyncProducer,
	topicOutgoing string,
	logger zerolog.Logger,
) error {
	type unsentRow struct {
		RecipientID string
		CampaignID  string
		Phone       string
		ClientID    string
		Source      string
		TemplateID  *string
	}

	rows, err := dbx.QueryContext(ctx, `
		SELECT cr.id, cr.campaign_id, cr.phone, c.client_id, c.source, c.template_id::text
		FROM campaign_recipients cr
		JOIN campaigns c ON c.id = cr.campaign_id
		WHERE cr.message_id IS NULL
		  AND cr.status = 'pending'
		  AND c.status = 'running'
		LIMIT 100`)
	if err != nil {
		return fmt.Errorf("query unsent recipients: %w", err)
	}
	defer rows.Close()

	renderer := smstpl.NewRenderer()
	var count int

	for rows.Next() {
		var r unsentRow
		if err := rows.Scan(&r.RecipientID, &r.CampaignID, &r.Phone, &r.ClientID, &r.Source, &r.TemplateID); err != nil {
			continue
		}

		clientUUID, _ := uuid.Parse(r.ClientID)
		msgID := uuid.New()
		now := time.Now()

		// Get template body and traffic_type
		var text, trafficType string
		if r.TemplateID != nil && *r.TemplateID != "" {
			_ = dbx.QueryRowContext(ctx, `SELECT body, COALESCE(traffic_type, 'transactional') FROM templates WHERE id = $1`, *r.TemplateID).Scan(&text, &trafficType)

			// Try to render with contact attributes
			var attrs []byte
			if err := dbx.QueryRowContext(ctx,
				`SELECT co.attributes FROM contacts co
				 JOIN campaign_recipients cr ON cr.contact_id = co.id
				 WHERE cr.id = $1`, r.RecipientID).Scan(&attrs); err == nil && len(attrs) > 0 {
				var bindings map[string]interface{}
				if json.Unmarshal(attrs, &bindings) == nil {
					if rendered, renderErr := renderer.Render(text, bindings); renderErr == nil {
						text = rendered
					}
				}
			}
		}

		// Insert message
		_, err := dbx.ExecContext(ctx, `
			INSERT INTO messages (
				id, source, destination, text, encoding, data_coding, esm_class,
				protocol_id, priority_flag, replace_if_present, registered_delivery,
				service_type, source_addr_ton, source_addr_npi, dest_addr_ton, dest_addr_npi,
				status, client_id, retry_count, max_retries, created_at, updated_at
			) VALUES ($1,$2,$3,$4,'GSM7',0,0,0,0,0,1,'',0,0,0,0,'queued',$5,0,5,$6,$6)`,
			msgID, r.Source, r.Phone, text, clientUUID, now,
		)
		if err != nil {
			logger.Error().Err(err).Str("phone", r.Phone).Msg("ошибка вставки сообщения при досылке")
			continue
		}

		if trafficType == "" {
			trafficType = "transactional"
		}

		// Publish to Kafka
		km := kafkaOutgoingMsg{
			ID: msgID.String(), MessageID: msgID,
			Source: r.Source, Destination: r.Phone, Text: text,
			ClientID: &clientUUID, MaxRetries: 5, CreatedAt: now,
			TrafficType: trafficType,
		}
		data, _ := json.Marshal(km)
		if _, _, kafkaErr := producer.SendMessage(&sarama.ProducerMessage{
			Topic: topicOutgoing,
			Value: sarama.ByteEncoder(data),
		}); kafkaErr != nil {
			logger.Error().Err(kafkaErr).Str("message_id", msgID.String()).Msg("ошибка публикации в Kafka при досылке")
			continue
		}

		// Update recipient with message_id
		_, _ = dbx.ExecContext(ctx,
			`UPDATE campaign_recipients SET message_id = $1, updated_at = now() WHERE id = $2`,
			msgID, r.RecipientID,
		)
		count++
	}

	if count > 0 {
		logger.Info().Int("count", count).Msg("досланы сообщения для получателей без message_id")
	}
	return nil
}

// staleRow holds the DB columns fetched for re-queuing.
type staleRow struct {
	ID          string
	Source      string
	Destination string
	Text        string
	ClientID    string
	CreatedAt   time.Time
}

// buildStaleKafkaMsg converts a staleRow into a kafkaOutgoingMsg ready for publishing.
// Extracted as a pure function so it can be unit-tested without DB.
func buildStaleKafkaMsg(row staleRow) (kafkaOutgoingMsg, error) {
	msgID, err := uuid.Parse(row.ID)
	if err != nil {
		return kafkaOutgoingMsg{}, fmt.Errorf("invalid message_id %q: %w", row.ID, err)
	}
	clientID, err := uuid.Parse(row.ClientID)
	if err != nil {
		return kafkaOutgoingMsg{}, fmt.Errorf("invalid client_id %q: %w", row.ClientID, err)
	}
	return kafkaOutgoingMsg{
		ID:          msgID.String(),
		MessageID:   msgID,
		Source:      row.Source,
		Destination: row.Destination,
		Text:        row.Text,
		ClientID:    &clientID,
		MaxRetries:  5,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// processStaleQueued finds messages in 'queued' state older than staleThreshold
// and re-publishes them to sms.outgoing. This recovers messages whose Kafka
// publish failed during materialization.
func processStaleQueued(
	ctx context.Context,
	dbx *sqlx.DB,
	producer sarama.SyncProducer,
	topicOutgoing string,
	staleThreshold time.Duration,
	logger zerolog.Logger,
) error {
	rows, err := dbx.QueryContext(ctx, `
		SELECT id::text, source, destination, text, client_id::text, created_at
		FROM messages
		WHERE status = 'queued'
		  AND updated_at < now() - ($1 * interval '1 second')
		LIMIT 100`,
		int64(staleThreshold.Seconds()),
	)
	if err != nil {
		return fmt.Errorf("query stale queued: %w", err)
	}
	defer rows.Close()

	var stale []staleRow
	for rows.Next() {
		var r staleRow
		if err := rows.Scan(&r.ID, &r.Source, &r.Destination, &r.Text, &r.ClientID, &r.CreatedAt); err != nil {
			logger.Error().Err(err).Msg("ошибка сканирования stale row")
			continue
		}
		stale = append(stale, r)
	}
	rows.Close()

	var published int
	for _, row := range stale {
		msg, err := buildStaleKafkaMsg(row)
		if err != nil {
			logger.Error().Err(err).Str("message_id", row.ID).Msg("ошибка построения Kafka сообщения для stale")
			continue
		}
		data, _ := json.Marshal(msg)
		if _, _, kafkaErr := producer.SendMessage(&sarama.ProducerMessage{
			Topic: topicOutgoing,
			Value: sarama.ByteEncoder(data),
		}); kafkaErr != nil {
			logger.Error().Err(kafkaErr).Str("message_id", row.ID).Msg("ошибка публикации stale сообщения")
			continue // do not refresh updated_at so it will be retried
		}
		// Refresh updated_at so we don't re-publish on the next tick.
		_, _ = dbx.ExecContext(ctx,
			`UPDATE messages SET updated_at = now() WHERE id = $1`, row.ID)
		published++
	}

	if published > 0 {
		logger.Info().Int("count", published).Msg("переотправлены зависшие queued сообщения")
	}
	return nil
}
