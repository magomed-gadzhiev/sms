package application

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
)

// E.164 format validation
var e164Regex = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)

// HLRService orchestrates HLR lookups with caching, provider failover, and logging
type HLRService struct {
	cache          domain.HLRCache
	providerRepo   domain.HLRProviderRepository
	logRepo        domain.LookupLogRepository
	adapterFactory AdapterFactory
	adapters       map[uuid.UUID]domain.HLRProviderAdapter
	adaptersMu     sync.RWMutex
	lookupTimeout  time.Duration
	logger         zerolog.Logger
}

// AdapterFactory creates HLR provider adapters
type AdapterFactory interface {
	CreateAdapter(provider *domain.HLRProvider) (domain.HLRProviderAdapter, error)
}

// NewHLRService creates a new HLR service
func NewHLRService(
	cache domain.HLRCache,
	providerRepo domain.HLRProviderRepository,
	logRepo domain.LookupLogRepository,
	adapterFactory AdapterFactory,
	lookupTimeout time.Duration,
) *HLRService {
	if lookupTimeout == 0 {
		lookupTimeout = 200 * time.Millisecond
	}
	return &HLRService{
		cache:          cache,
		providerRepo:   providerRepo,
		logRepo:        logRepo,
		adapterFactory: adapterFactory,
		adapters:       make(map[uuid.UUID]domain.HLRProviderAdapter),
		lookupTimeout:  lookupTimeout,
		logger:         log.With().Str("component", "hlr-service").Logger(),
	}
}

// ValidateMSISDN checks if a number is in valid E.164 format
func ValidateMSISDN(msisdn string) bool {
	cleaned := strings.TrimPrefix(msisdn, "+")
	return e164Regex.MatchString(cleaned)
}

// IsShortCode returns true if the number is a short code (not suitable for HLR)
func IsShortCode(msisdn string) bool {
	cleaned := strings.TrimPrefix(msisdn, "+")
	return len(cleaned) <= 6
}

// LookupNumber performs an HLR lookup with caching and provider failover
func (s *HLRService) LookupNumber(ctx context.Context, msisdn string, forceRefresh bool, clientID uuid.UUID, requestID string, source domain.LookupSource, messageID *uuid.UUID) (*domain.LookupResult, error) {
	start := time.Now()

	// Normalize MSISDN - strip leading +
	cleanMSISDN := strings.TrimPrefix(msisdn, "+")

	// Validate format
	if !ValidateMSISDN(cleanMSISDN) {
		return nil, domain.ErrInvalidMSISDN
	}

	// Skip HLR for short codes
	if IsShortCode(cleanMSISDN) {
		return nil, nil // caller should fallback to prefix routing
	}

	// Check cache (unless force refresh)
	if !forceRefresh {
		cached, err := s.cache.Get(ctx, cleanMSISDN)
		if err != nil {
			s.logger.Warn().Err(err).Str("msisdn", cleanMSISDN).Msg("ошибка чтения кеша HLR")
		}
		if cached != nil {
			latencyMs := int(time.Since(start).Milliseconds())
			// Log the lookup
			go s.logLookup(context.Background(), cached, source, clientID, requestID, messageID, latencyMs)
			return cached, nil
		}
	}

	// Cache miss — query HLR providers
	result, err := s.queryProviders(ctx, cleanMSISDN)
	if err != nil {
		latencyMs := int(time.Since(start).Milliseconds())
		s.logger.Warn().Err(err).
			Str("msisdn", cleanMSISDN).
			Int("latency_ms", latencyMs).
			Msg("HLR lookup не удался, fallback на prefix routing")
		return nil, err // caller should fallback
	}

	// Store in cache
	if cacheErr := s.cache.Set(ctx, cleanMSISDN, result); cacheErr != nil {
		s.logger.Warn().Err(cacheErr).Str("msisdn", cleanMSISDN).Msg("ошибка записи в кеш HLR")
	}

	latencyMs := int(time.Since(start).Milliseconds())
	// Log the lookup
	go s.logLookup(context.Background(), result, source, clientID, requestID, messageID, latencyMs)

	s.logger.Debug().
		Str("msisdn", cleanMSISDN).
		Str("operator", result.OperatorMCCMNC).
		Str("status", string(result.NumberStatus)).
		Bool("ported", result.IsPorted).
		Int("latency_ms", latencyMs).
		Msg("HLR lookup выполнен")

	return result, nil
}

// queryProviders tries HLR providers by priority with failover
func (s *HLRService) queryProviders(ctx context.Context, msisdn string) (*domain.LookupResult, error) {
	// Extract country code from MSISDN for provider selection
	// Simple heuristic: first 1-3 digits are country code
	countryCode := extractCountryCode(msisdn)

	var providers []*domain.HLRProvider
	var err error

	if countryCode != "" {
		providers, err = s.providerRepo.GetByPriority(ctx, countryCode)
	}
	if err != nil || len(providers) == 0 {
		// Fallback: get all active providers
		providers, err = s.providerRepo.ListActive(ctx)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения HLR провайдеров: %w", err)
		}
	}

	if len(providers) == 0 {
		return nil, domain.ErrHLRProviderUnavailable
	}

	// Try providers by priority
	for _, provider := range providers {
		if !provider.IsAvailable() {
			continue
		}

		adapter, err := s.getOrCreateAdapter(provider)
		if err != nil {
			s.logger.Warn().Err(err).
				Str("provider", provider.Name).
				Msg("ошибка создания адаптера HLR")
			continue
		}

		// Create timeout context for this lookup
		lookupCtx, cancel := context.WithTimeout(ctx, s.lookupTimeout)
		result, err := adapter.Lookup(lookupCtx, msisdn)
		cancel()

		if err != nil {
			s.logger.Warn().Err(err).
				Str("provider", provider.Name).
				Str("msisdn", msisdn).
				Msg("HLR запрос не удался, пробуем следующего провайдера")
			provider.RecordFailure()
			continue
		}

		result.HLRProviderID = &provider.ID
		provider.RecordSuccess()
		return result, nil
	}

	return nil, domain.ErrHLRLookupFailed
}

// getOrCreateAdapter gets or creates an adapter for a provider
func (s *HLRService) getOrCreateAdapter(provider *domain.HLRProvider) (domain.HLRProviderAdapter, error) {
	s.adaptersMu.RLock()
	adapter, exists := s.adapters[provider.ID]
	s.adaptersMu.RUnlock()

	if exists {
		return adapter, nil
	}

	s.adaptersMu.Lock()
	defer s.adaptersMu.Unlock()

	// Double-check after acquiring write lock
	if adapter, exists = s.adapters[provider.ID]; exists {
		return adapter, nil
	}

	adapter, err := s.adapterFactory.CreateAdapter(provider)
	if err != nil {
		return nil, err
	}

	s.adapters[provider.ID] = adapter
	return adapter, nil
}

// logLookup writes a lookup log entry (fire-and-forget)
func (s *HLRService) logLookup(ctx context.Context, result *domain.LookupResult, source domain.LookupSource, clientID uuid.UUID, requestID string, messageID *uuid.UUID, latencyMs int) {
	entry := domain.NewLookupLogEntry(result, source, clientID, requestID, messageID, latencyMs)
	if err := s.logRepo.Insert(ctx, entry); err != nil {
		s.logger.Warn().Err(err).Msg("ошибка записи в lookup_log")
	}
}

// GetAdapters возвращает карту адаптеров HLR провайдеров (для health monitor)
func (s *HLRService) GetAdapters() map[uuid.UUID]domain.HLRProviderAdapter {
	return s.adapters
}

// GetAdaptersMu возвращает мьютекс адаптеров (для health monitor)
func (s *HLRService) GetAdaptersMu() *sync.RWMutex {
	return &s.adaptersMu
}

// extractCountryCode extracts the country code from an E.164 number
// This is a simplified heuristic; production systems would use a comprehensive mapping
func extractCountryCode(msisdn string) string {
	// Common country codes by prefix length
	if len(msisdn) < 7 {
		return ""
	}
	// 1-digit: 1 (US/CA), 7 (RU/KZ)
	switch msisdn[0] {
	case '1':
		return "US"
	case '7':
		return "RU"
	}
	// 2-digit codes
	if len(msisdn) >= 2 {
		prefix2 := msisdn[:2]
		switch prefix2 {
		case "33":
			return "FR"
		case "34":
			return "ES"
		case "39":
			return "IT"
		case "44":
			return "GB"
		case "49":
			return "DE"
		case "38":
			if len(msisdn) >= 3 && msisdn[2] == '0' {
				return "UA"
			}
		case "37":
			if len(msisdn) >= 3 {
				switch msisdn[2] {
				case '5':
					return "BY"
				}
			}
		}
	}
	return ""
}

// HealthMonitor manages health checks for HLR providers
type HealthMonitor struct {
	providerRepo domain.HLRProviderRepository
	adapters     map[uuid.UUID]domain.HLRProviderAdapter
	adaptersMu   *sync.RWMutex
	factory      AdapterFactory
	interval     time.Duration
	stopCh       chan struct{}
	logger       zerolog.Logger
}

// NewHealthMonitor creates a new HLR provider health monitor
func NewHealthMonitor(
	providerRepo domain.HLRProviderRepository,
	adapters map[uuid.UUID]domain.HLRProviderAdapter,
	adaptersMu *sync.RWMutex,
	factory AdapterFactory,
	interval time.Duration,
) *HealthMonitor {
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &HealthMonitor{
		providerRepo: providerRepo,
		adapters:     adapters,
		adaptersMu:   adaptersMu,
		factory:      factory,
		interval:     interval,
		stopCh:       make(chan struct{}),
		logger:       log.With().Str("component", "hlr-health-monitor").Logger(),
	}
}

// Start begins the health check loop (goroutine-safe, follows Start/Stop pattern from constitution)
func (m *HealthMonitor) Start() {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		m.logger.Info().
			Dur("interval", m.interval).
			Msg("HLR health monitor запущен")

		for {
			select {
			case <-ticker.C:
				m.checkAll()
			case <-m.stopCh:
				m.logger.Info().Msg("HLR health monitor остановлен")
				return
			}
		}
	}()
}

// Stop stops the health check loop
func (m *HealthMonitor) Stop() {
	close(m.stopCh)
}

// checkAll pings all active providers and updates their status
func (m *HealthMonitor) checkAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providers, err := m.providerRepo.ListActive(ctx)
	if err != nil {
		m.logger.Error().Err(err).Msg("ошибка получения провайдеров для health check")
		return
	}

	for _, provider := range providers {
		m.adaptersMu.RLock()
		adapter, exists := m.adapters[provider.ID]
		m.adaptersMu.RUnlock()

		if !exists {
			a, err := m.factory.CreateAdapter(provider)
			if err != nil {
				m.logger.Warn().Err(err).Str("provider", provider.Name).Msg("ошибка создания адаптера для health check")
				continue
			}
			m.adaptersMu.Lock()
			m.adapters[provider.ID] = a
			m.adaptersMu.Unlock()
			adapter = a
		}

		pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
		pingErr := adapter.Ping(pingCtx)
		pingCancel()

		if pingErr != nil {
			provider.RecordFailure()
			m.logger.Warn().Err(pingErr).
				Str("provider", provider.Name).
				Str("status", string(provider.Status)).
				Msg("HLR провайдер health check не пройден")
		} else {
			provider.RecordSuccess()
		}

		// Persist status update
		if updateErr := m.providerRepo.Update(ctx, provider); updateErr != nil {
			m.logger.Warn().Err(updateErr).Str("provider", provider.Name).Msg("ошибка обновления статуса провайдера")
		}

		// Обновляем метрику здоровья провайдера
		healthValue := 1.0
		if provider.Status == domain.HLRProviderStatusDegraded {
			healthValue = 0.5
		} else if provider.Status == domain.HLRProviderStatusUnhealthy || provider.Status == domain.HLRProviderStatusDisabled {
			healthValue = 0
		}
		infrastructure.HLRProviderHealth.WithLabelValues(provider.Name).Set(healthValue)
	}

	// Проверяем, есть ли хоть один доступный провайдер
	if len(providers) > 0 {
		allUnavailable := true
		for _, provider := range providers {
			if provider.IsAvailable() {
				allUnavailable = false
				break
			}
		}

		if allUnavailable {
			infrastructure.HLRAllProvidersDown.Set(1)
			m.logger.Error().
				Int("total_providers", len(providers)).
				Msg("ALERT: все HLR провайдеры недоступны, маршрутизация переключена на prefix-based fallback")
		} else {
			infrastructure.HLRAllProvidersDown.Set(0)
		}
	}
}
