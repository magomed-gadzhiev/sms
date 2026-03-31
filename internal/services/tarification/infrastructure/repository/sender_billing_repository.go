package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// SenderBillingRepository реализует domain.SenderBillingRepository
type SenderBillingRepository struct {
	db *sqlx.DB
}

// NewSenderBillingRepository создает новый репозиторий billing-записей имён отправителей
func NewSenderBillingRepository(db *sqlx.DB) *SenderBillingRepository {
	return &SenderBillingRepository{db: db}
}

// Create создаёт billing record; при дубликате возвращает ошибку
func (r *SenderBillingRepository) Create(ctx context.Context, record *domain.SenderNameBillingRecord) error {
	query := `
		INSERT INTO sender_name_billing_records (
			id, sender_registration_id, client_id, operator_id,
			billing_month, amount, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		record.ID,
		record.SenderRegistrationID,
		record.ClientID,
		record.OperatorID,
		record.BillingMonth,
		record.Amount,
		record.CreatedAt,
	)
	return err
}

// CreateIfNotExists использует INSERT ON CONFLICT DO NOTHING для идемпотентности.
// Возвращает (true, nil) если запись была создана, (false, nil) если уже существовала.
func (r *SenderBillingRepository) CreateIfNotExists(ctx context.Context, record *domain.SenderNameBillingRecord) (bool, error) {
	query := `
		INSERT INTO sender_name_billing_records (
			id, sender_registration_id, client_id, operator_id,
			billing_month, amount, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (sender_registration_id, billing_month) DO NOTHING
	`
	result, err := r.db.ExecContext(ctx, query,
		record.ID,
		record.SenderRegistrationID,
		record.ClientID,
		record.OperatorID,
		record.BillingMonth,
		record.Amount,
		record.CreatedAt,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ListByRegistration возвращает все billing-записи по регистрации, DESC по billing_month
func (r *SenderBillingRepository) ListByRegistration(ctx context.Context, regID uuid.UUID, limit, offset int) ([]*domain.SenderNameBillingRecord, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sender_name_billing_records WHERE sender_registration_id = $1`, regID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, sender_registration_id, client_id, operator_id, billing_month, amount, created_at
		FROM sender_name_billing_records
		WHERE sender_registration_id = $1
		ORDER BY billing_month DESC
		LIMIT $2 OFFSET $3
	`)

	rows, err := r.db.QueryContext(ctx, query, regID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []*domain.SenderNameBillingRecord
	for rows.Next() {
		var rec domain.SenderNameBillingRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.SenderRegistrationID,
			&rec.ClientID,
			&rec.OperatorID,
			&rec.BillingMonth,
			&rec.Amount,
			&rec.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		records = append(records, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// ListByClient возвращает все billing-записи по клиенту за период
func (r *SenderBillingRepository) ListByClient(ctx context.Context, clientID uuid.UUID, from, to time.Time, limit, offset int) ([]*domain.SenderNameBillingRecord, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sender_name_billing_records WHERE client_id = $1 AND billing_month >= $2 AND billing_month <= $3`,
		clientID, from, to,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, sender_registration_id, client_id, operator_id, billing_month, amount, created_at
		FROM sender_name_billing_records
		WHERE client_id = $1 AND billing_month >= $2 AND billing_month <= $3
		ORDER BY billing_month DESC
		LIMIT $4 OFFSET $5
	`

	rows, err := r.db.QueryContext(ctx, query, clientID, from, to, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []*domain.SenderNameBillingRecord
	for rows.Next() {
		var rec domain.SenderNameBillingRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.SenderRegistrationID,
			&rec.ClientID,
			&rec.OperatorID,
			&rec.BillingMonth,
			&rec.Amount,
			&rec.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		records = append(records, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
