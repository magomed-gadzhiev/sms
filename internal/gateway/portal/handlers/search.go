package handlers

import (
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// SearchHandlers содержит handlers для глобального поиска
type SearchHandlers struct {
	pool *pgxpool.Pool
}

// NewSearchHandlers создает SearchHandlers
func NewSearchHandlers(pool *pgxpool.Pool) *SearchHandlers {
	return &SearchHandlers{pool: pool}
}

type commandItem struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	URL      string `json:"url"`
}

// Search обрабатывает GET /search?q=...
func (h *SearchHandlers) Search(w http.ResponseWriter, r *http.Request) {
	log.Debug().Str("q", r.URL.Query().Get("q")).Msg("поиск")

	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	clientID, _ := middleware.GetClientID(r.Context())

	q := r.URL.Query().Get("q")
	if len([]rune(q)) < 2 {
		respondError(w, shared.ErrInvalidInput("Минимальная длина запроса — 2 символа"))
		return
	}

	if h.pool == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"items": []interface{}{}})
		return
	}

	ctx := r.Context()
	pattern := "%" + q + "%"

	var (
		mu      sync.Mutex
		results []commandItem
		wg      sync.WaitGroup
	)

	// Search templates
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := h.pool.Query(ctx,
			`SELECT id, name FROM templates WHERE client_id = $1 AND name ILIKE $2 LIMIT 3`,
			clientID, pattern,
		)
		if err != nil {
			log.Error().Err(err).Msg("ошибка поиска шаблонов")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				continue
			}
			mu.Lock()
			results = append(results, commandItem{
				ID: id, Type: "template", Category: "Шаблоны",
				Title: name, URL: "/templates",
			})
			mu.Unlock()
		}
	}()

	// Search sender names
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := h.pool.Query(ctx,
			`SELECT id, name FROM sender_names WHERE client_id = $1 AND name ILIKE $2 LIMIT 3`,
			clientID, pattern,
		)
		if err != nil {
			log.Error().Err(err).Msg("ошибка поиска имён отправителей")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				continue
			}
			mu.Lock()
			results = append(results, commandItem{
				ID: id, Type: "sender_name", Category: "Имена отправителей",
				Title: name, URL: "/sender-names",
			})
			mu.Unlock()
		}
	}()

	// Search contact lists
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := h.pool.Query(ctx,
			`SELECT id, name FROM contact_lists WHERE client_id = $1 AND name ILIKE $2 LIMIT 3`,
			clientID, pattern,
		)
		if err != nil {
			log.Error().Err(err).Msg("ошибка поиска контактных баз")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				continue
			}
			mu.Lock()
			results = append(results, commandItem{
				ID: id, Type: "contact_list", Category: "Контактные базы",
				Title: name, URL: "/contact-lists",
			})
			mu.Unlock()
		}
	}()

	wg.Wait()

	if results == nil {
		results = []commandItem{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"items": results})
}
