package application

import "testing"

func TestTarifyUnified_HappyPathApproves(t *testing.T) {
	t.Skip("Task 8b: implement happy-path test with fakes from price_resolver_test.go + new stubs (stubSubUsage, stubLogRepo, stubMarginLog, stubSaga, stubOperatorLookup).")
}

func TestTarifyUnified_FallbackOnNotFound(t *testing.T) {
	t.Skip("Task 8b: fakeRuleRepo.err = ErrNoApplicableRule → fallbackReason=\"not_found\".")
}

func TestTarifyUnified_FallbackOnCalcError(t *testing.T) {
	t.Skip("Task 8b: malformed TiersJSON → fallbackReason=\"calc_error\".")
}

func TestTarifyUnified_ChargeFailReturnsRejection(t *testing.T) {
	t.Skip("Task 8b: stubSaga returns Success=false → Approved=false, no fallback.")
}

func TestTarifyUnified_IdempotencyShortCircuits(t *testing.T) {
	t.Skip("Task 8b: stubLogRepo.existing != nil → returns existing response, no resolve/charge.")
}

func TestTarifyUnified_OperatorLookupError_Fallback(t *testing.T) {
	t.Skip("Task 8b: stubOperatorLookup returns ErrOperatorNotFound → fallbackReason=\"operator_lookup_error\".")
}
