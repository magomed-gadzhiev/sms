package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Client представляет клиента системы
type Client struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	Name          string          `json:"name" db:"name"`
	APIKey        string          `json:"api_key" db:"api_key"`
	Secret        string          `json:"secret" db:"secret"`
	Email         string          `json:"email" db:"email"`
	ContactPerson string          `json:"contact_person" db:"contact_person"`
	Phone         string          `json:"phone" db:"phone"`
	Active        bool            `json:"active" db:"active"`
	Metadata      json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`

	// Суб-аккаунты
	ParentClientID *uuid.UUID `json:"parent_client_id,omitempty" db:"parent_client_id"`
	IsReseller     bool       `json:"is_reseller" db:"is_reseller"`
	MaxSubAccounts int        `json:"max_sub_accounts" db:"max_sub_accounts"`

	PlanID            *uuid.UUID `json:"plan_id,omitempty" db:"plan_id"`
	MonthlySMSCount   int       `json:"monthly_sms_count" db:"monthly_sms_count"`
	MonthlySMSResetAt time.Time `json:"monthly_sms_reset_at" db:"monthly_sms_reset_at"`
	IsSandbox         bool      `json:"is_sandbox" db:"is_sandbox"`

	// Billing mode for sub-accounts
	BillingMode          string  `json:"billing_mode" db:"billing_mode"`
	SpendingLimitMonthly *string `json:"spending_limit_monthly,omitempty" db:"spending_limit_monthly"`
	SpendingLimitDaily   *string `json:"spending_limit_daily,omitempty" db:"spending_limit_daily"`

	// Связи
	Config *ClientConfig `json:"config,omitempty" db:"-"`
	Plan   *Plan         `json:"plan,omitempty" db:"-"` // loaded via JOIN
}

// IsActive проверяет, активен ли клиент
func (c *Client) IsActive() bool {
	return c.Active
}

// GetMetadata возвращает метаданные как map
func (c *Client) GetMetadata() map[string]string {
	if c.Metadata == nil || len(c.Metadata) == 0 {
		return make(map[string]string)
	}

	var metadata map[string]string
	if err := json.Unmarshal(c.Metadata, &metadata); err != nil {
		return make(map[string]string)
	}

	return metadata
}

// IsSubAccount проверяет, является ли клиент суб-аккаунтом
func (c *Client) IsSubAccount() bool {
	return c.ParentClientID != nil
}

// CanCreateSubAccount проверяет, может ли клиент создать ещё один суб-аккаунт
func (c *Client) CanCreateSubAccount(currentCount int) bool {
	return c.IsReseller && currentCount < c.MaxSubAccounts
}

func (c *Client) IsWithinMonthlyQuota(additionalMessages int) bool {
	if c.Plan == nil {
		return true
	}
	return c.MonthlySMSCount+additionalMessages <= c.Plan.MaxSMSPerMonth
}

func (c *Client) RemainingMonthlyQuota() int {
	if c.Plan == nil {
		return 0
	}
	remaining := c.Plan.MaxSMSPerMonth - c.MonthlySMSCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (c *Client) NeedsMonthlyReset() bool {
	return time.Now().After(c.MonthlySMSResetAt)
}

// SetMetadata устанавливает метаданные из map
func (c *Client) SetMetadata(metadata map[string]string) error {
	if metadata == nil {
		c.Metadata = json.RawMessage("{}")
		return nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	c.Metadata = json.RawMessage(data)
	return nil
}
