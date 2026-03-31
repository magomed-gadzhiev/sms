package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

const reachabilityRedisTTL = 1 * time.Hour

// OperatorSupportWithPrefixRepo — расширенный интерфейс репозитория с поддержкой lookup по префиксу
type OperatorSupportWithPrefixRepo interface {
	domain.OperatorSupportRepository
	GetOperatorIDByPrefix(ctx context.Context, msisdn string) (uuid.UUID, error)
}

// MaxMessengerChecker — интерфейс для проверки регистрации MSISDN в Max Messenger
type MaxMessengerChecker interface {
	CheckRegistration(ctx context.Context, msisdn string, cfg map[string]interface{}) (bool, error)
}

// ReachabilityService проверяет доступность канала для получателя
type ReachabilityService struct {
	ocs                OperatorSupportWithPrefixRepo
	redis              *redis.Client
	routingClient      routingv1.RoutingServiceClient
	logger             zerolog.Logger
	maxMessengerChecker MaxMessengerChecker
	channelRepo        domain.ChannelRepository
}

func NewReachabilityService(
	ocs domain.OperatorSupportRepository,
	redisClient *redis.Client,
	routingConn *grpc.ClientConn,
	logger zerolog.Logger,
) *ReachabilityService {
	var prefixRepo OperatorSupportWithPrefixRepo
	if r, ok := ocs.(OperatorSupportWithPrefixRepo); ok {
		prefixRepo = r
	}

	return &ReachabilityService{
		ocs:           prefixRepo,
		redis:         redisClient,
		routingClient: routingv1.NewRoutingServiceClient(routingConn),
		logger:        logger.With().Str("component", "reachability_service").Logger(),
	}
}

// SetMaxMessengerChecker устанавливает checker для Max Messenger reachability
func (s *ReachabilityService) SetMaxMessengerChecker(checker MaxMessengerChecker, channelRepo domain.ChannelRepository) {
	s.maxMessengerChecker = checker
	s.channelRepo = channelRepo
}

// CheckReachability проверяет доступность channelType для msisdn
func (s *ReachabilityService) CheckReachability(ctx context.Context, msisdn string, channelType domain.ChannelType) (bool, error) {
	cacheKey := fmt.Sprintf("reachability:%s:%s", msisdn, channelType)

	// 1. Проверить Redis кеш
	val, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		supported := val == "1"
		s.logger.Debug().
			Str("msisdn", msisdn).
			Str("channel_type", string(channelType)).
			Bool("supported", supported).
			Msg("reachability из кеша")
		return supported, nil
	}

	// 2. Для Max Messenger — проверить регистрацию MSISDN через API
	if channelType == domain.ChannelMaxMessenger && s.maxMessengerChecker != nil {
		return s.checkMaxMessengerReachability(ctx, msisdn, cacheKey)
	}

	// 3. Определить operator_id из MSISDN
	operatorID, err := s.resolveOperatorID(ctx, msisdn)
	if err != nil {
		// При ошибке - считаем канал доступным (не блокируем)
		s.logger.Warn().Err(err).Str("msisdn", msisdn).Msg("не удалось определить оператора, считаем доступным")
		return true, nil
	}

	if operatorID == uuid.Nil {
		// Оператор не найден - SMS доступен по умолчанию
		supported := channelType == domain.ChannelSMS
		s.cacheResult(ctx, cacheKey, supported)
		return supported, nil
	}

	// 3. Проверить поддержку канала оператором
	ocs, err := s.ocs.GetSupport(ctx, operatorID, channelType)
	if err != nil {
		s.logger.Warn().Err(err).
			Str("operator_id", operatorID.String()).
			Str("channel_type", string(channelType)).
			Msg("ошибка получения поддержки канала, считаем доступным")
		return true, nil
	}

	s.cacheResult(ctx, cacheKey, ocs.Supported)
	s.logger.Debug().
		Str("msisdn", msisdn).
		Str("channel_type", string(channelType)).
		Str("operator_id", operatorID.String()).
		Bool("supported", ocs.Supported).
		Msg("reachability проверена")

	return ocs.Supported, nil
}

func (s *ReachabilityService) checkMaxMessengerReachability(ctx context.Context, msisdn, cacheKey string) (bool, error) {
	// Загрузить конфиг канала Max Messenger
	ch, err := s.channelRepo.GetByType(ctx, domain.ChannelMaxMessenger)
	if err != nil {
		s.logger.Warn().Err(err).Msg("не удалось загрузить конфиг Max Messenger, fail-open")
		return true, nil
	}

	registered, err := s.maxMessengerChecker.CheckRegistration(ctx, msisdn, ch.Config)
	if err != nil {
		// fail-open: при ошибке считаем доступным
		s.logger.Warn().Err(err).Str("msisdn", msisdn).Msg("ошибка проверки регистрации Max Messenger, fail-open")
		return true, nil
	}

	s.cacheResult(ctx, cacheKey, registered)
	return registered, nil
}

func (s *ReachabilityService) resolveOperatorID(ctx context.Context, msisdn string) (uuid.UUID, error) {
	// Сначала пробуем через prefix repo (быстрее)
	if s.ocs != nil {
		operatorID, err := s.ocs.GetOperatorIDByPrefix(ctx, msisdn)
		if err == nil && operatorID != uuid.Nil {
			return operatorID, nil
		}
	}

	// Fallback: через routing service NumberLookup - получаем operator_name,
	// затем ищем в таблице operators по имени (не идеально, но работает)
	// Здесь упрощённо возвращаем nil UUID, что приведёт к дефолтному поведению
	normalized := strings.TrimPrefix(msisdn, "+")
	resp, err := s.routingClient.NumberLookup(ctx, &routingv1.NumberLookupRequest{
		Msisdn: normalized,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("routing number lookup: %w", err)
	}
	if resp.OperatorMccmnc == "" {
		return uuid.Nil, nil
	}

	// MCCMNC известен, но без прямого маппинга на UUID - возвращаем Nil
	// (В реальной реализации нужна таблица mccmnc → operator_id)
	s.logger.Debug().
		Str("msisdn", msisdn).
		Str("operator_mccmnc", resp.OperatorMccmnc).
		Msg("оператор определён по MCCMNC, UUID не определён без prefix table")
	return uuid.Nil, nil
}

func (s *ReachabilityService) cacheResult(ctx context.Context, key string, supported bool) {
	val := "0"
	if supported {
		val = "1"
	}
	if err := s.redis.Set(ctx, key, val, reachabilityRedisTTL).Err(); err != nil {
		s.logger.Warn().Err(err).Str("key", key).Msg("ошибка кеширования reachability")
	}
}
