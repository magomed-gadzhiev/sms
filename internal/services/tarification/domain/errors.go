package domain

import "errors"

var (
	// SenderRegistration errors
	ErrSenderRegistrationNotFound     = errors.New("sender registration not found")
	ErrSenderRegistrationDuplicate    = errors.New("sender registration already exists for this client, operator, and name")
	ErrOperatorUnsupportedSenderType  = errors.New("operator does not support this sender registration type")
	ErrSenderRegistrationInvalidType  = errors.New("invalid sender registration type, must be 'paid' or 'free'")
	ErrSenderRegistrationInvalidStatus = errors.New("invalid sender registration status")

	// TariffPlan errors
	ErrTariffPlanNotFound           = errors.New("tariff plan not found")
	ErrTariffPlanDuplicate          = errors.New("active tariff plan already exists for this operator and sender category")
	ErrTariffPlanInvalidStrategy    = errors.New("invalid tarification strategy")
	ErrTariffPlanInvalidCategory    = errors.New("invalid sender category")
	ErrTariffPlanHasActivePeriod    = errors.New("cannot deactivate tariff plan with active period")

	// TariffPeriod errors
	ErrTariffPeriodNotFound         = errors.New("tariff period not found")
	ErrTariffPeriodOverlap          = errors.New("tariff period overlaps with existing period")
	ErrTariffPeriodInvalidDates     = errors.New("end date must be after start date")

	// TariffTier errors
	ErrTariffTierNotFound           = errors.New("tariff tier not found")
	ErrTariffTierDuplicate          = errors.New("tariff tier with this from_count already exists")
	ErrTariffTierPeriodActive       = errors.New("cannot modify tiers in active period")
	ErrTariffTierInvalidPrice       = errors.New("price per segment must be non-negative")

	// PricingPeriod errors
	ErrPricingPeriodNotFound        = errors.New("pricing period not found")
	ErrPricingPeriodOverlap         = errors.New("pricing period overlaps with existing period")
	ErrPricingPeriodOutOfBounds     = errors.New("pricing period must be within tariff period bounds")

	// PrepaidFee errors
	ErrPrepaidFeeNotFound           = errors.New("prepaid fee not found")
	ErrPrepaidFeeAlreadyCharged     = errors.New("prepaid fee already charged")

	// UsageCounter errors
	ErrUsageCounterNotFound         = errors.New("usage counter not found")

	// Tarification errors
	ErrNoActiveTariffPlan           = errors.New("no active tariff plan for operator and sender category")
	ErrNoActivePeriod               = errors.New("no active tariff period for current date")
	ErrInsufficientBalance          = errors.New("insufficient balance")
	ErrIdempotencyKeyExists         = errors.New("tarification already processed for this idempotency key")
	ErrUnknownOperator              = errors.New("operator not determined for destination number")
	ErrDebtBlocking                 = errors.New("client has outstanding debt, sending blocked")

	// Unified pricing model (Phase 1)
	ErrNoApplicableRule  = errors.New("no applicable price rule found")
	ErrOwnerIDMismatch   = errors.New("owner_id must be NULL iff owner_type=platform")
	ErrPriceSpecMismatch = errors.New("price_value/tiers_json does not match price_model")
	ErrInvalidPeriod     = errors.New("valid_to must be after valid_from")
)
