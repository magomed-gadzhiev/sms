package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)


// TransactionRepository реализует domain.TransactionRepository
type TransactionRepository struct {
	db *sqlx.DB
}

// NewTransactionRepository создает новый репозиторий транзакций
func NewTransactionRepository(db *sqlx.DB) *TransactionRepository {
	return &TransactionRepository{
		db: db,
	}
}

// Create создает новую транзакцию
func (r *TransactionRepository) Create(ctx context.Context, transaction *domain.Transaction) error {
	query := `
		INSERT INTO transactions (
			id, client_id, type, amount, currency,
			balance_before, balance_after, description,
			message_id, payment_method, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`

	var metadataJSON []byte
	if transaction.Metadata != nil && len(transaction.Metadata) > 0 {
		var err error
		metadataJSON, err = json.Marshal(transaction.Metadata)
		if err != nil {
			return err
		}
	}

	_, err := r.db.ExecContext(ctx, query,
		transaction.ID,
		transaction.ClientID,
		string(transaction.Type),
		transaction.Amount,
		transaction.Currency,
		transaction.BalanceBefore,
		transaction.BalanceAfter,
		transaction.Description,
		transaction.MessageID,
		transaction.PaymentMethod,
		metadataJSON,
		transaction.CreatedAt,
	)

	return err
}

// GetByID получает транзакцию по ID
func (r *TransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	var transaction domain.Transaction
	var metadataJSON []byte
	var messageID sql.NullString

	query := `
		SELECT id, client_id, type, amount, currency,
			balance_before, balance_after, description,
			message_id, payment_method, metadata, created_at
		FROM transactions
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&transaction.ID,
		&transaction.ClientID,
		&transaction.Type,
		&transaction.Amount,
		&transaction.Currency,
		&transaction.BalanceBefore,
		&transaction.BalanceAfter,
		&transaction.Description,
		&messageID,
		&transaction.PaymentMethod,
		&metadataJSON,
		&transaction.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, err
	}

	if messageID.Valid {
		msgID, err := uuid.Parse(messageID.String)
		if err == nil {
			transaction.MessageID = &msgID
		}
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &transaction.Metadata); err != nil {
			transaction.Metadata = make(map[string]interface{})
		}
	} else {
		transaction.Metadata = make(map[string]interface{})
	}

	return &transaction, nil
}

// GetAll получает все транзакции с пагинацией (для админа)
func (r *TransactionRepository) GetAll(ctx context.Context, limit, offset int) ([]*domain.Transaction, error) {
	query := `
		SELECT id, client_id, type, amount, currency,
			balance_before, balance_after, description,
			message_id, payment_method, metadata, created_at
		FROM transactions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	return r.scanTransactions(ctx, query, limit, offset)
}

// GetByClientID получает транзакции по client_id с пагинацией
func (r *TransactionRepository) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Transaction, error) {
	query := `
		SELECT id, client_id, type, amount, currency,
			balance_before, balance_after, description,
			message_id, payment_method, metadata, created_at
		FROM transactions
		WHERE client_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.scanTransactions(ctx, query, clientID, limit, offset)
}

// GetByClientIDAndType получает транзакции по client_id и типу
func (r *TransactionRepository) GetByClientIDAndType(ctx context.Context, clientID uuid.UUID, transactionType domain.TransactionType, limit, offset int) ([]*domain.Transaction, error) {
	query := `
		SELECT id, client_id, type, amount, currency,
			balance_before, balance_after, description,
			message_id, payment_method, metadata, created_at
		FROM transactions
		WHERE client_id = $1 AND type = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	return r.scanTransactions(ctx, query, clientID, string(transactionType), limit, offset)
}

// GetByClientIDAndPeriod получает транзакции за период
func (r *TransactionRepository) GetByClientIDAndPeriod(ctx context.Context, clientID uuid.UUID, from, to time.Time, limit, offset int) ([]*domain.Transaction, error) {
	query := `
		SELECT id, client_id, type, amount, currency,
			balance_before, balance_after, description,
			message_id, payment_method, metadata, created_at
		FROM transactions
		WHERE client_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`

	return r.scanTransactions(ctx, query, clientID, from, to, limit, offset)
}

// GetByMessageID получает транзакцию по message_id
func (r *TransactionRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) (*domain.Transaction, error) {
	var transaction domain.Transaction
	var metadataJSON []byte
	var msgID sql.NullString

	query := `
		SELECT id, client_id, type, amount, currency,
			balance_before, balance_after, description,
			message_id, payment_method, metadata, created_at
		FROM transactions
		WHERE message_id = $1
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&transaction.ID,
		&transaction.ClientID,
		&transaction.Type,
		&transaction.Amount,
		&transaction.Currency,
		&transaction.BalanceBefore,
		&transaction.BalanceAfter,
		&transaction.Description,
		&msgID,
		&transaction.PaymentMethod,
		&metadataJSON,
		&transaction.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, err
	}

	if msgID.Valid {
		parsedID, err := uuid.Parse(msgID.String)
		if err == nil {
			transaction.MessageID = &parsedID
		}
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &transaction.Metadata); err != nil {
			transaction.Metadata = make(map[string]interface{})
		}
	} else {
		transaction.Metadata = make(map[string]interface{})
	}

	return &transaction, nil
}

// scanTransactions сканирует результаты запроса в транзакции
func (r *TransactionRepository) scanTransactions(ctx context.Context, query string, args ...interface{}) ([]*domain.Transaction, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*domain.Transaction
	for rows.Next() {
		var transaction domain.Transaction
		var metadataJSON []byte
		var messageID sql.NullString

		err := rows.Scan(
			&transaction.ID,
			&transaction.ClientID,
			&transaction.Type,
			&transaction.Amount,
			&transaction.Currency,
			&transaction.BalanceBefore,
			&transaction.BalanceAfter,
			&transaction.Description,
			&messageID,
			&transaction.PaymentMethod,
			&metadataJSON,
			&transaction.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if messageID.Valid {
			msgID, err := uuid.Parse(messageID.String)
			if err == nil {
				transaction.MessageID = &msgID
			}
		}

		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &transaction.Metadata); err != nil {
				transaction.Metadata = make(map[string]interface{})
			}
		} else {
			transaction.Metadata = make(map[string]interface{})
		}

		transactions = append(transactions, &transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}
