package domain

import "errors"

var (
	// Ошибки Account
	ErrAccountNotFound          = errors.New("account not found")
	ErrAccountClientIDRequired  = errors.New("client_id is required")
	ErrAccountCurrencyRequired  = errors.New("currency is required")
	ErrInsufficientBalance      = errors.New("insufficient balance")
	ErrAccountAlreadyFrozen     = errors.New("account is already frozen")
	ErrAccountNotFrozen         = errors.New("account is not frozen")

	// Ошибки Transaction
	ErrTransactionNotFound         = errors.New("transaction not found")
	ErrTransactionClientIDRequired = errors.New("client_id is required")
	ErrTransactionInvalidType      = errors.New("invalid transaction type")
	ErrTransactionAmountRequired   = errors.New("amount is required")
	ErrTransactionCurrencyRequired = errors.New("currency is required")

	// Ошибки PricingRule
	ErrPricingRuleNotFound         = errors.New("pricing rule not found")
	ErrPricingRulePatternRequired  = errors.New("destination_pattern is required")
	ErrPricingRulePriceRequired    = errors.New("price_per_message is required")
	ErrPricingRuleCurrencyRequired = errors.New("currency is required")
	ErrPricingRulePriorityInvalid  = errors.New("priority must be non-negative")
)
