package max_messenger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// ReachabilityChecker проверяет регистрацию MSISDN в Max Messenger
type ReachabilityChecker struct {
	httpClient *http.Client
	logger     zerolog.Logger
	metrics    *MaxMessengerMetrics
}

// NewReachabilityChecker создаёт новый ReachabilityChecker
func NewReachabilityChecker(logger zerolog.Logger, metrics *MaxMessengerMetrics) *ReachabilityChecker {
	return &ReachabilityChecker{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		logger:     logger.With().Str("component", "max_messenger_reachability").Logger(),
		metrics:    metrics,
	}
}

// CheckRegistration проверяет, зарегистрирован ли MSISDN в Max Messenger.
// Fail-open: при любой ошибке возвращает true (считаем доступным).
func (c *ReachabilityChecker) CheckRegistration(ctx context.Context, msisdn string, cfg map[string]interface{}) (bool, error) {
	checkURL, _ := cfg["check_registration_url"].(string)
	if checkURL == "" {
		// Нет URL проверки — fail-open
		return true, nil
	}

	apiKey, _ := cfg["api_key"].(string)

	payload := CheckRegistrationRequest{MSISDN: msisdn}
	body, err := json.Marshal(payload)
	if err != nil {
		c.metrics.ReachabilityChecks.WithLabelValues("error").Inc()
		c.logger.Warn().Err(err).Msg("ошибка маршалинга запроса проверки регистрации")
		return true, nil // fail-open
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, checkURL, bytes.NewReader(body))
	if err != nil {
		c.metrics.ReachabilityChecks.WithLabelValues("error").Inc()
		return true, nil // fail-open
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.metrics.ReachabilityChecks.WithLabelValues("error").Inc()
		c.logger.Warn().Err(err).Str("msisdn", msisdn).Msg("ошибка запроса проверки регистрации, fail-open")
		return true, nil // fail-open
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		c.metrics.ReachabilityChecks.WithLabelValues("error").Inc()
		c.logger.Warn().
			Int("status_code", resp.StatusCode).
			Str("body", string(respBody)).
			Msg("ошибка API проверки регистрации, fail-open")
		return true, nil // fail-open
	}

	var result CheckRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.metrics.ReachabilityChecks.WithLabelValues("error").Inc()
		c.logger.Warn().Err(err).Msg("ошибка декодирования ответа проверки регистрации, fail-open")
		return true, nil // fail-open
	}

	if result.Registered {
		c.metrics.ReachabilityChecks.WithLabelValues("registered").Inc()
	} else {
		c.metrics.ReachabilityChecks.WithLabelValues("not_registered").Inc()
	}

	c.logger.Debug().
		Str("msisdn", msisdn).
		Bool("registered", result.Registered).
		Msg("проверка регистрации в Max Messenger")

	return result.Registered, nil
}
