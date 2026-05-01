package middleware

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/api/http/response"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ResellerOnlyMiddleware ограничивает доступ к роутам /reseller/* только
// клиентами с is_reseller=true. Закрывает BUG-82 этапа 27/30: до фикса
// группа network-statistics handlers в этом же subrouter'е (statistics,
// analytics-summary, monitoring, drilldown, export, views) не вызывала
// handler-level checkReseller, поэтому sub-account и обычный user видели
// агрегированную статистику parent'а через /reseller/* endpoint'ы.
//
// Middleware дублирует проверку для уже-защищённых handlers
// (reseller_dashboard, reseller_routing, reseller_tariffs и т.д.) — это
// сознательная цена за защиту от регрессии: новые endpoint'ы под
// /reseller/* автоматически получают guard, без необходимости помнить
// добавить checkReseller в каждом новом handler.
//
// Сообщение об ошибке совпадает с handler-level checkReseller
// ("доступ только для агрегаторов") — frontend и e2e-тесты опираются на
// текст; менять формулировку — отдельный i18n PR.
//
// Возвращаем 403 Forbidden (а не 401 Unauthorized) — пользователь уже
// аутентифицирован, не хватает только роли. До C.5 здесь был 401, что
// заставляло api/client.ts редиректить sub-account на /login при попытке
// открыть /reseller/* — бесконечный цикл (юзер уже залогинен).
func ResellerOnlyMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// pool может быть nil, если portal-gateway не смог подключиться
			// к Postgres на старте (см. cmd/portal-gateway/main.go:134-137 —
			// ошибка только логируется как Warn). Fail-closed: без БД мы не
			// можем достоверно проверить is_reseller → отказываем в доступе.
			if pool == nil {
				response.Error(w, shared.ErrInternalServer("База недоступна"))
				return
			}

			clientID, ok := GetClientID(r.Context())
			if !ok {
				response.Error(w, shared.ErrUnauthorized("Клиент не найден"))
				return
			}

			var isReseller bool
			err := pool.QueryRow(r.Context(),
				`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
			).Scan(&isReseller)
			if err != nil || !isReseller {
				response.Error(w, shared.ErrForbidden("доступ только для агрегаторов"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
