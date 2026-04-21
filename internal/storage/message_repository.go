package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// MessageRepository предоставляет методы для работы с сообщениями
type MessageRepository struct {
	db *sqlx.DB
}

// NewMessageRepository создает новый репозиторий сообщений
func NewMessageRepository(db *DB) *MessageRepository {
	return &MessageRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает новое сообщение
func (r *MessageRepository) Create(ctx context.Context, msg *shared.Message) error {
	// template_id / sender_name_id plumb audit linkage on the non-Kafka paths
	// (scheduled dispatch, sandbox fast-path) which call this repo directly.
	// UpdateStatus is set-once elsewhere; these fields do not mutate after
	// insert, so UPDATE paths don't need them.
	query := `
		INSERT INTO messages (
			id, message_id, external_id, source, destination, text, encoding,
			data_coding, esm_class, protocol_id, priority_flag, replace_if_present,
			registered_delivery, validity_period, service_type,
			source_addr_ton, source_addr_npi, dest_addr_ton, dest_addr_npi,
			status, status_message, provider_id, route_id, client_id,
			retry_count, max_retries, next_retry_at, smpp_message_id,
			submitted_at, delivered_at, failed_at, created_at, updated_at, scheduled_at,
			template_id, sender_name_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28,
			$29, $30, $31, $32, $33, $34, $35, $36
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.MessageID, msg.ExternalID, msg.Source, msg.Destination,
		msg.Text, msg.Encoding, msg.DataCoding, msg.ESMClass, msg.ProtocolID,
		msg.PriorityFlag, msg.ReplaceIfPresent, msg.RegisteredDelivery,
		msg.ValidityPeriod, msg.ServiceType, msg.SourceAddrTON, msg.SourceAddrNPI,
		msg.DestAddrTON, msg.DestAddrNPI, msg.Status, msg.StatusMessage,
		msg.ProviderID, msg.RouteID, msg.ClientID, msg.RetryCount, msg.MaxRetries,
		msg.NextRetryAt, msg.SMPPMessageID, msg.SubmittedAt, msg.DeliveredAt,
		msg.FailedAt, msg.CreatedAt, msg.UpdatedAt, msg.ScheduledAt,
		msg.TemplateID, msg.SenderNameID,
	)

	return err
}

// GetByID получает сообщение по ID
func (r *MessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
	var msg shared.Message
	query := `
		SELECT * FROM messages WHERE id = $1
	`

	err := r.db.GetContext(ctx, &msg, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &msg, nil
}

// GetByMessageID получает сообщение по message_id
func (r *MessageRepository) GetByMessageID(ctx context.Context, messageID string) (*shared.Message, error) {
	var msg shared.Message
	query := `
		SELECT * FROM messages WHERE message_id = $1
	`

	err := r.db.GetContext(ctx, &msg, query, messageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &msg, nil
}

// GetByExternalID получает сообщение по external_id
func (r *MessageRepository) GetByExternalID(ctx context.Context, externalID string) (*shared.Message, error) {
	var msg shared.Message
	query := `
		SELECT * FROM messages WHERE external_id = $1
	`

	err := r.db.GetContext(ctx, &msg, query, externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &msg, nil
}

// GetBySMPPMessageID получает сообщение по SMPP message_id от провайдера
func (r *MessageRepository) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*shared.Message, error) {
	var msg shared.Message
	query := `
		SELECT * FROM messages WHERE smpp_message_id = $1
	`

	err := r.db.GetContext(ctx, &msg, query, smppMessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &msg, nil
}

// Update обновляет сообщение
func (r *MessageRepository) Update(ctx context.Context, msg *shared.Message) error {
	query := `
		UPDATE messages SET
			message_id = $2, external_id = $3, source = $4, destination = $5,
			text = $6, encoding = $7, data_coding = $8, esm_class = $9,
			protocol_id = $10, priority_flag = $11, replace_if_present = $12,
			registered_delivery = $13, validity_period = $14, service_type = $15,
			source_addr_ton = $16, source_addr_npi = $17, dest_addr_ton = $18,
			dest_addr_npi = $19, status = $20, status_message = $21,
			provider_id = $22, route_id = $23, client_id = $24,
			retry_count = $25, max_retries = $26, next_retry_at = $27,
			smpp_message_id = $28, submitted_at = $29, delivered_at = $30,
			failed_at = $31, updated_at = $32
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.MessageID, msg.ExternalID, msg.Source, msg.Destination,
		msg.Text, msg.Encoding, msg.DataCoding, msg.ESMClass, msg.ProtocolID,
		msg.PriorityFlag, msg.ReplaceIfPresent, msg.RegisteredDelivery,
		msg.ValidityPeriod, msg.ServiceType, msg.SourceAddrTON, msg.SourceAddrNPI,
		msg.DestAddrTON, msg.DestAddrNPI, msg.Status, msg.StatusMessage,
		msg.ProviderID, msg.RouteID, msg.ClientID, msg.RetryCount, msg.MaxRetries,
		msg.NextRetryAt, msg.SMPPMessageID, msg.SubmittedAt, msg.DeliveredAt,
		msg.FailedAt, msg.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateStatus обновляет статус сообщения
func (r *MessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error {
	now := time.Now()
	var setFields []string
	var args []interface{}
	argIndex := 1

	setFields = append(setFields, fmt.Sprintf("status = $%d", argIndex))
	args = append(args, status)
	argIndex++

	if statusMessage != "" {
		setFields = append(setFields, fmt.Sprintf("status_message = $%d", argIndex))
		args = append(args, statusMessage)
		argIndex++
	}

	// Обновляем соответствующие timestamp поля
	switch status {
	case shared.MessageStatusSent:
		setFields = append(setFields, fmt.Sprintf("submitted_at = $%d", argIndex))
		args = append(args, now)
		argIndex++
	case shared.MessageStatusDelivered:
		setFields = append(setFields, fmt.Sprintf("delivered_at = $%d", argIndex))
		args = append(args, now)
		argIndex++
	case shared.MessageStatusFailed, shared.MessageStatusExpired, shared.MessageStatusRejected:
		setFields = append(setFields, fmt.Sprintf("failed_at = $%d", argIndex))
		args = append(args, now)
		argIndex++
	}

	setFields = append(setFields, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, now)
	argIndex++

	args = append(args, id)
	finalArgIndex := argIndex

	query := fmt.Sprintf(`
		UPDATE messages SET %s WHERE id = $%d
	`, joinSetFields(setFields, ", "), finalArgIndex)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// GetPendingForRetry получает сообщения, готовые для повторной попытки
func (r *MessageRepository) GetPendingForRetry(ctx context.Context, limit int) ([]*shared.Message, error) {
	var messages []*shared.Message
	query := `
		SELECT * FROM messages
		WHERE status = $1
			AND retry_count < max_retries
			AND next_retry_at IS NOT NULL
			AND next_retry_at <= $2
		ORDER BY next_retry_at ASC
		LIMIT $3
	`

	err := r.db.SelectContext(ctx, &messages, query,
		shared.MessageStatusFailed, time.Now(), limit)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// GetByClientID получает сообщения клиента
func (r *MessageRepository) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error) {
	var messages []*shared.Message
	query := `
		SELECT id, COALESCE(message_id, '') as message_id, COALESCE(external_id, '') as external_id,
			source, destination, text, COALESCE(encoding, 'GSM7') as encoding,
			COALESCE(data_coding, 0) as data_coding, COALESCE(esm_class, 0) as esm_class,
			COALESCE(protocol_id, 0) as protocol_id, COALESCE(priority_flag, 0) as priority_flag,
			COALESCE(replace_if_present, 0) as replace_if_present,
			COALESCE(registered_delivery, 1) as registered_delivery,
			validity_period, COALESCE(service_type, '') as service_type,
			COALESCE(source_addr_ton, 0) as source_addr_ton, COALESCE(source_addr_npi, 0) as source_addr_npi,
			COALESCE(dest_addr_ton, 0) as dest_addr_ton, COALESCE(dest_addr_npi, 0) as dest_addr_npi,
			COALESCE(status, 'pending') as status, COALESCE(status_message, '') as status_message,
			provider_id, route_id, client_id,
			COALESCE(retry_count, 0) as retry_count, COALESCE(max_retries, 5) as max_retries,
			next_retry_at, COALESCE(smpp_message_id, '') as smpp_message_id,
			submitted_at, delivered_at, failed_at, created_at, updated_at,
			scheduled_at, COALESCE(segment_count, 1) as segment_count, expired_at
		FROM messages
		WHERE client_id = $1
	`
	args := []interface{}{clientID}
	argIndex := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	query += fmt.Sprintf(`
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, argIndex, argIndex+1)
	args = append(args, limit, offset)

	err := r.db.SelectContext(ctx, &messages, query, args...)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// GetAll получает все сообщения с фильтрацией
func (r *MessageRepository) GetAll(ctx context.Context, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error) {
	var messages []*shared.Message
	baseQuery := `SELECT id, COALESCE(message_id, '') as message_id, COALESCE(external_id, '') as external_id,
		source, destination, text, COALESCE(encoding, 'GSM7') as encoding,
		COALESCE(data_coding, 0) as data_coding, COALESCE(esm_class, 0) as esm_class,
		COALESCE(protocol_id, 0) as protocol_id, COALESCE(priority_flag, 0) as priority_flag,
		COALESCE(replace_if_present, 0) as replace_if_present,
		COALESCE(registered_delivery, 1) as registered_delivery,
		validity_period, COALESCE(service_type, '') as service_type,
		COALESCE(source_addr_ton, 0) as source_addr_ton, COALESCE(source_addr_npi, 0) as source_addr_npi,
		COALESCE(dest_addr_ton, 0) as dest_addr_ton, COALESCE(dest_addr_npi, 0) as dest_addr_npi,
		COALESCE(status, 'pending') as status, COALESCE(status_message, '') as status_message,
		provider_id, route_id, client_id,
		COALESCE(retry_count, 0) as retry_count, COALESCE(max_retries, 5) as max_retries,
		next_retry_at, COALESCE(smpp_message_id, '') as smpp_message_id,
		submitted_at, delivered_at, failed_at, created_at, updated_at,
		scheduled_at, COALESCE(segment_count, 1) as segment_count, expired_at
		FROM messages`
	query := baseQuery
	args := []interface{}{}
	argIndex := 1

	if status != nil {
		query += fmt.Sprintf(" WHERE status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	query += fmt.Sprintf(`
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, argIndex, argIndex+1)
	args = append(args, limit, offset)

	err := r.db.SelectContext(ctx, &messages, query, args...)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// GetByDestination получает сообщения по номеру получателя
func (r *MessageRepository) GetByDestination(ctx context.Context, destination string, limit, offset int) ([]*shared.Message, error) {
	var messages []*shared.Message
	query := `
		SELECT * FROM messages
		WHERE destination = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	err := r.db.SelectContext(ctx, &messages, query, destination, limit, offset)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// IncrementRetryCount увеличивает счетчик попыток и устанавливает следующую попытку
func (r *MessageRepository) IncrementRetryCount(ctx context.Context, id uuid.UUID, nextRetryAt time.Time) error {
	query := `
		UPDATE messages SET
			retry_count = retry_count + 1,
			next_retry_at = $2,
			updated_at = $3
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, nextRetryAt, time.Now())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// GetScheduledReady fetches messages ready for scheduled delivery
func (r *MessageRepository) GetScheduledReady(ctx context.Context, limit int) ([]*shared.Message, error) {
	var messages []*shared.Message
	query := `SELECT * FROM messages
		WHERE status = 'scheduled'
		AND scheduled_at <= NOW()
		AND created_at >= NOW() - INTERVAL '7 days'
		ORDER BY scheduled_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	err := r.db.SelectContext(ctx, &messages, query, limit)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// GetStuckPending fetches messages stuck in pending with scheduled_at set
func (r *MessageRepository) GetStuckPending(ctx context.Context, threshold time.Duration, limit int) ([]*shared.Message, error) {
	var messages []*shared.Message
	thresholdTime := time.Now().Add(-threshold)
	query := `SELECT * FROM messages
		WHERE status = 'pending'
		AND scheduled_at IS NOT NULL
		AND updated_at < $1
		AND created_at >= NOW() - INTERVAL '7 days'
		ORDER BY updated_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED`

	err := r.db.SelectContext(ctx, &messages, query, thresholdTime, limit)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// ListMessages returns paginated messages for a client with full filter support and total count.
func (r *MessageRepository) ListMessages(ctx context.Context, clientID uuid.UUID, filter shared.MessageFilter) ([]*shared.Message, int, error) {
	baseSelect := `SELECT id, COALESCE(message_id, '') as message_id, COALESCE(external_id, '') as external_id,
		source, destination, text, COALESCE(encoding, 'GSM7') as encoding,
		COALESCE(data_coding, 0) as data_coding, COALESCE(esm_class, 0) as esm_class,
		COALESCE(protocol_id, 0) as protocol_id, COALESCE(priority_flag, 0) as priority_flag,
		COALESCE(replace_if_present, 0) as replace_if_present,
		COALESCE(registered_delivery, 1) as registered_delivery,
		validity_period, COALESCE(service_type, '') as service_type,
		COALESCE(source_addr_ton, 0) as source_addr_ton, COALESCE(source_addr_npi, 0) as source_addr_npi,
		COALESCE(dest_addr_ton, 0) as dest_addr_ton, COALESCE(dest_addr_npi, 0) as dest_addr_npi,
		COALESCE(status, 'pending') as status, COALESCE(status_message, '') as status_message,
		provider_id, route_id, client_id,
		COALESCE(retry_count, 0) as retry_count, COALESCE(max_retries, 5) as max_retries,
		next_retry_at, COALESCE(smpp_message_id, '') as smpp_message_id,
		submitted_at, delivered_at, failed_at, created_at, updated_at,
		scheduled_at, COALESCE(segment_count, 1) as segment_count, expired_at
		FROM messages`

	where := " WHERE client_id = $1"
	args := []interface{}{clientID}
	idx := 2

	if filter.Status != nil {
		where += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, *filter.Status)
		idx++
	}
	if filter.Destination != "" {
		where += fmt.Sprintf(" AND destination = $%d", idx)
		args = append(args, filter.Destination)
		idx++
	}
	if filter.From != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", idx)
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", idx)
		args = append(args, *filter.To)
		idx++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM messages" + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count messages: %w", err)
	}

	listQuery := baseSelect + where + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, filter.Limit, filter.Offset)

	messages := make([]*shared.Message, 0)
	if err := r.db.SelectContext(ctx, &messages, listQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("list messages: %w", err)
	}

	return messages, total, nil
}

// ListScheduled returns paginated scheduled messages for a client with total count.
func (r *MessageRepository) ListScheduled(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*shared.Message, int, error) {
	countQuery := `SELECT COUNT(*) FROM messages WHERE client_id = $1 AND status = 'scheduled'`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, clientID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count scheduled messages: %w", err)
	}

	messages := make([]*shared.Message, 0)
	query := `SELECT id, COALESCE(message_id, '') as message_id, COALESCE(external_id, '') as external_id,
		source, destination, text, COALESCE(encoding, 'GSM7') as encoding,
		COALESCE(data_coding, 0) as data_coding, COALESCE(esm_class, 0) as esm_class,
		COALESCE(protocol_id, 0) as protocol_id, COALESCE(priority_flag, 0) as priority_flag,
		COALESCE(replace_if_present, 0) as replace_if_present,
		COALESCE(registered_delivery, 1) as registered_delivery,
		validity_period, COALESCE(service_type, '') as service_type,
		COALESCE(source_addr_ton, 0) as source_addr_ton, COALESCE(source_addr_npi, 0) as source_addr_npi,
		COALESCE(dest_addr_ton, 0) as dest_addr_ton, COALESCE(dest_addr_npi, 0) as dest_addr_npi,
		COALESCE(status, 'pending') as status, COALESCE(status_message, '') as status_message,
		provider_id, route_id, client_id,
		COALESCE(retry_count, 0) as retry_count, COALESCE(max_retries, 5) as max_retries,
		next_retry_at, COALESCE(smpp_message_id, '') as smpp_message_id,
		submitted_at, delivered_at, failed_at, created_at, updated_at,
		scheduled_at, COALESCE(segment_count, 1) as segment_count, expired_at
		FROM messages
		WHERE client_id = $1 AND status = 'scheduled'
		ORDER BY scheduled_at ASC
		LIMIT $2 OFFSET $3`

	if err := r.db.SelectContext(ctx, &messages, query, clientID, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("list scheduled messages: %w", err)
	}

	return messages, total, nil
}

// CancelByIDAndStatus atomically cancels a scheduled message
func (r *MessageRepository) CancelByIDAndStatus(ctx context.Context, id, clientID uuid.UUID) error {
	query := `UPDATE messages SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND client_id = $2 AND status = 'scheduled'`

	result, err := r.db.ExecContext(ctx, query, id, clientID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("message not found or not in scheduled status")
	}

	return nil
}

// GetSentExpired fetches messages in "sent" status older than timeout (no DLR received)
func (r *MessageRepository) GetSentExpired(ctx context.Context, timeout time.Duration, limit int) ([]*shared.Message, error) {
	var messages []*shared.Message
	cutoff := time.Now().Add(-timeout)
	query := `SELECT * FROM messages
		WHERE status = 'sent'
		AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED`

	err := r.db.SelectContext(ctx, &messages, query, cutoff, limit)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// BulkUpdateStatusToExpired updates a batch of messages to expired status
func (r *MessageRepository) BulkUpdateStatusToExpired(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	now := time.Now()
	query := `UPDATE messages
		SET status = 'expired', expired_at = $1, updated_at = $1
		WHERE id = ANY($2)`

	_, err := r.db.ExecContext(ctx, query, now, pq.Array(ids))
	return err
}

// UpdateStatusByMessageID обновляет статус сообщения по message_id
func (r *MessageRepository) UpdateStatusByMessageID(ctx context.Context, messageID string, status shared.MessageStatus) error {
	now := time.Now()
	var setFields []string
	var args []interface{}
	argIndex := 1

	setFields = append(setFields, fmt.Sprintf("status = $%d", argIndex))
	args = append(args, status)
	argIndex++

	// Обновляем соответствующие timestamp поля
	switch status {
	case shared.MessageStatusSent:
		setFields = append(setFields, fmt.Sprintf("submitted_at = $%d", argIndex))
		args = append(args, now)
		argIndex++
	case shared.MessageStatusDelivered:
		setFields = append(setFields, fmt.Sprintf("delivered_at = $%d", argIndex))
		args = append(args, now)
		argIndex++
	case shared.MessageStatusFailed, shared.MessageStatusExpired, shared.MessageStatusRejected:
		setFields = append(setFields, fmt.Sprintf("failed_at = $%d", argIndex))
		args = append(args, now)
		argIndex++
	}

	setFields = append(setFields, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, now)
	argIndex++

	args = append(args, messageID)
	finalArgIndex := argIndex

	query := fmt.Sprintf(`
		UPDATE messages SET %s WHERE message_id = $%d
	`, joinSetFields(setFields, ", "), finalArgIndex)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateTextByMessageID обновляет текст сообщения по message_id
func (r *MessageRepository) UpdateTextByMessageID(ctx context.Context, messageID string, text string) error {
	now := time.Now()
	query := `
		UPDATE messages SET text = $1, updated_at = $2 WHERE message_id = $3
	`

	result, err := r.db.ExecContext(ctx, query, text, now, messageID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// joinSetFields вспомогательная функция для объединения SET полей
func joinSetFields(fields []string, sep string) string {
	return strings.Join(fields, sep)
}
