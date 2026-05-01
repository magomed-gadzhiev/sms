package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)


// PricingRuleRepository реализует domain.PricingRuleRepository
type PricingRuleRepository struct {
	db *sqlx.DB
}

// NewPricingRuleRepository создает новый репозиторий правил тарификации
func NewPricingRuleRepository(db *sqlx.DB) *PricingRuleRepository {
	return &PricingRuleRepository{
		db: db,
	}
}

// Create создает новое правило тарификации
func (r *PricingRuleRepository) Create(ctx context.Context, rule *domain.PricingRule) error {
	query := `
		INSERT INTO pricing_rules (
			id, client_id, destination_pattern, price_per_message,
			currency, priority, active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		rule.ID,
		rule.ClientID,
		rule.DestinationPattern,
		rule.PricePerMessage,
		rule.Currency,
		rule.Priority,
		rule.Active,
		rule.CreatedAt,
		rule.UpdatedAt,
	)

	return err
}

// GetByID получает правило по ID
func (r *PricingRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PricingRule, error) {
	var rule domain.PricingRule
	var clientID sql.NullString

	query := `
		SELECT id, client_id, destination_pattern, price_per_message,
			currency, priority, active, created_at, updated_at
		FROM pricing_rules
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rule.ID,
		&clientID,
		&rule.DestinationPattern,
		&rule.PricePerMessage,
		&rule.Currency,
		&rule.Priority,
		&rule.Active,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPricingRuleNotFound
		}
		return nil, err
	}

	if clientID.Valid {
		id, err := uuid.Parse(clientID.String)
		if err == nil {
			rule.ClientID = &id
		}
	}

	return &rule, nil
}

// GetByClientID получает правила для клиента
func (r *PricingRuleRepository) GetByClientID(ctx context.Context, clientID *uuid.UUID, activeOnly bool) ([]*domain.PricingRule, error) {
	var query string
	var args []interface{}

	if clientID == nil {
		// Глобальные правила
		if activeOnly {
			query = `
				SELECT id, client_id, destination_pattern, price_per_message,
					currency, priority, active, created_at, updated_at
				FROM pricing_rules
				WHERE client_id IS NULL AND active = true
				ORDER BY priority DESC, created_at ASC
			`
		} else {
			query = `
				SELECT id, client_id, destination_pattern, price_per_message,
					currency, priority, active, created_at, updated_at
				FROM pricing_rules
				WHERE client_id IS NULL
				ORDER BY priority DESC, created_at ASC
			`
		}
	} else {
		if activeOnly {
			query = `
				SELECT id, client_id, destination_pattern, price_per_message,
					currency, priority, active, created_at, updated_at
				FROM pricing_rules
				WHERE client_id = $1 AND active = true
				ORDER BY priority DESC, created_at ASC
			`
			args = []interface{}{*clientID}
		} else {
			query = `
				SELECT id, client_id, destination_pattern, price_per_message,
					currency, priority, active, created_at, updated_at
				FROM pricing_rules
				WHERE client_id = $1
				ORDER BY priority DESC, created_at ASC
			`
			args = []interface{}{*clientID}
		}
	}

	return r.scanRules(ctx, query, args...)
}

// GetGlobalRules получает глобальные правила
func (r *PricingRuleRepository) GetGlobalRules(ctx context.Context, activeOnly bool) ([]*domain.PricingRule, error) {
	return r.GetByClientID(ctx, nil, activeOnly)
}

// GetMatchingRule получает правило, подходящее для номера назначения
func (r *PricingRuleRepository) GetMatchingRule(ctx context.Context, clientID *uuid.UUID, destination string) (*domain.PricingRule, error) {
	// Сначала проверяем клиентские правила, затем глобальные
	var rules []*domain.PricingRule
	var err error

	if clientID != nil {
		// Получаем активные правила для клиента
		rules, err = r.GetByClientID(ctx, clientID, true)
		if err != nil {
			return nil, err
		}

		// Проверяем каждое правило на соответствие паттерну
		for _, rule := range rules {
			if matches, _ := regexp.MatchString(rule.DestinationPattern, destination); matches {
				return rule, nil
			}
		}
	}

	// Если не нашли клиентское правило, проверяем глобальные
	globalRules, err := r.GetGlobalRules(ctx, true)
	if err != nil {
		return nil, err
	}

	for _, rule := range globalRules {
		if matches, _ := regexp.MatchString(rule.DestinationPattern, destination); matches {
			return rule, nil
		}
	}

	return nil, domain.ErrPricingRuleNotFound
}

// Update обновляет правило
func (r *PricingRuleRepository) Update(ctx context.Context, rule *domain.PricingRule) error {
	query := `
		UPDATE pricing_rules
		SET destination_pattern = $1, price_per_message = $2,
			currency = $3, priority = $4, active = $5, updated_at = $6
		WHERE id = $7
	`

	result, err := r.db.ExecContext(ctx, query,
		rule.DestinationPattern,
		rule.PricePerMessage,
		rule.Currency,
		rule.Priority,
		rule.Active,
		rule.UpdatedAt,
		rule.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrPricingRuleNotFound
	}

	return nil
}

// Delete удаляет правило
func (r *PricingRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM pricing_rules WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrPricingRuleNotFound
	}

	return nil
}

// scanRules сканирует результаты запроса в правила
func (r *PricingRuleRepository) scanRules(ctx context.Context, query string, args ...interface{}) ([]*domain.PricingRule, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*domain.PricingRule
	for rows.Next() {
		var rule domain.PricingRule
		var clientID sql.NullString

		err := rows.Scan(
			&rule.ID,
			&clientID,
			&rule.DestinationPattern,
			&rule.PricePerMessage,
			&rule.Currency,
			&rule.Priority,
			&rule.Active,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if clientID.Valid {
			id, err := uuid.Parse(clientID.String)
			if err == nil {
				rule.ClientID = &id
			}
		}

		rules = append(rules, &rule)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}
