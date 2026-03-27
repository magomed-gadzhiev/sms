package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
)

type domainClientContextKey string

const DomainClientKey domainClientContextKey = "domain_client"

// GetDomainClient извлекает domain.Client из контекста (устанавливается расширенным auth middleware).
func GetDomainClient(ctx context.Context) (*domain.Client, bool) {
	c, ok := ctx.Value(DomainClientKey).(*domain.Client)
	return c, ok
}

// QuotaMiddleware проверяет месячную квоту SMS для клиента.
// Требует, чтобы в контексте был domain.Client с загруженным Plan (устанавливается
// расширенным auth middleware — Task 7).
//
// Текущее состояние: auth middleware (LOAD TEST MODE) не кладёт domain.Client в контекст,
// поэтому middleware работает как pass-through и логирует предупреждение.
//
// TODO: когда auth middleware начнёт класть domain.Client с Plan в контекст
// (ключ DomainClientKey), quota check заработает автоматически.
func QuotaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, ok := GetDomainClient(r.Context())
		if !ok || client == nil {
			// domain.Client ещё не загружается auth middleware — пропускаем.
			log.Ctx(r.Context()).Debug().
				Msg("quota check skipped: domain client not in context (auth middleware stub active)")
			next.ServeHTTP(w, r)
			return
		}

		if client.Plan == nil {
			// Plan не загружен — пропускаем проверку квоты.
			log.Ctx(r.Context()).Warn().
				Str("client_id", client.ID.String()).
				Msg("quota check skipped: plan not loaded for client")
			next.ServeHTTP(w, r)
			return
		}

		if !client.IsWithinMonthlyQuota(1) {
			log.Ctx(r.Context()).Warn().
				Str("client_id", client.ID.String()).
				Str("plan_id", client.PlanID.String()).
				Int("monthly_sms_count", client.MonthlySMSCount).
				Int("plan_max_sms", client.Plan.MaxSMSPerMonth).
				Msg("monthly SMS quota exceeded")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "QUOTA_EXCEEDED",
					"message": "Monthly SMS quota exceeded. Please upgrade your plan.",
				},
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
