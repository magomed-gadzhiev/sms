package application

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type QuotaService struct {
	repo domain.AggregatorQuotaRepository
}

func NewQuotaService(repo domain.AggregatorQuotaRepository) *QuotaService {
	return &QuotaService{repo: repo}
}

type QuotaConsumeResult struct {
	NoQuota             bool
	HasOverage          bool
	OverageChargeAmount string
	Currency            string
	QuotaID             uuid.UUID
}

func (s *QuotaService) ConsumeQuota(ctx context.Context, aggregatorID uuid.UUID, segments int) (*QuotaConsumeResult, error) {
	quota, err := s.repo.GetActive(ctx, aggregatorID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("get active quota: %w", err)
	}
	if quota == nil {
		return &QuotaConsumeResult{NoQuota: true}, nil
	}

	incr, err := s.repo.IncrementUsage(ctx, quota.ID, segments)
	if err != nil {
		return nil, fmt.Errorf("increment quota: %w", err)
	}

	result := &QuotaConsumeResult{
		QuotaID:  quota.ID,
		Currency: quota.Currency,
	}

	if incr.OverageCount > 0 {
		result.HasOverage = true
		result.OverageChargeAmount = multiplyPriceInt(incr.OverageRate, incr.OverageCount)
	} else {
		result.OverageChargeAmount = "0"
	}

	return result, nil
}

func (s *QuotaService) CreateQuota(ctx context.Context, aggregatorID uuid.UUID, periodStart, periodEnd time.Time, segmentLimit int64, overageRate, currency string, autoRenew bool) (*domain.AggregatorQuota, error) {
	quota := &domain.AggregatorQuota{
		ID:           uuid.New(),
		AggregatorID: aggregatorID,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		SegmentLimit: segmentLimit,
		OverageRate:  overageRate,
		Currency:     currency,
		AutoRenew:    autoRenew,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.repo.Create(ctx, quota); err != nil {
		return nil, err
	}
	return quota, nil
}

func (s *QuotaService) GetActiveQuota(ctx context.Context, aggregatorID uuid.UUID) (*domain.AggregatorQuota, error) {
	return s.repo.GetActive(ctx, aggregatorID, time.Now())
}

func (s *QuotaService) UpdateQuota(ctx context.Context, quotaID uuid.UUID, segmentLimit int64, overageRate string, autoRenew bool) (*domain.AggregatorQuota, error) {
	quota, err := s.repo.GetByID(ctx, quotaID)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, fmt.Errorf("quota not found")
	}
	quota.SegmentLimit = segmentLimit
	quota.OverageRate = overageRate
	quota.AutoRenew = autoRenew
	if err := s.repo.Update(ctx, quota); err != nil {
		return nil, err
	}
	return quota, nil
}

func (s *QuotaService) ListQuotas(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]*domain.AggregatorQuota, int, error) {
	return s.repo.ListByAggregator(ctx, aggregatorID, limit, offset)
}

func multiplyPriceInt(price string, count int) string {
	p, ok := new(big.Float).SetString(price)
	if !ok {
		return "0"
	}
	c := new(big.Float).SetInt64(int64(count))
	result := new(big.Float).Mul(p, c)
	return result.Text('f', 2)
}
