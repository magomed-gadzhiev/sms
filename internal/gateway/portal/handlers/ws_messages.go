package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/sse"
)

var wsUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		// Allow same-host connections. Behind a reverse proxy the Host header
		// reflects the public host (set by nginx proxy_set_header Host $host),
		// so compare against both http and https variants.
		host := r.Host
		return origin == "https://"+host || origin == "http://"+host
	},
}

// WsMessagesHandlers handles WebSocket connections for the live message feed
type WsMessagesHandlers struct {
	hub    *sse.Hub
	dbPool *pgxpool.Pool
}

// NewWsMessagesHandlers creates WsMessagesHandlers
func NewWsMessagesHandlers(hub *sse.Hub, dbPool *pgxpool.Pool) *WsMessagesHandlers {
	return &WsMessagesHandlers{hub: hub, dbPool: dbPool}
}

type wsMessageEvent struct {
	MessageID    string `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	Status       string `json:"status"`
	PhoneMasked  string `json:"phone_masked"`
	Operator     string `json:"operator"`
	Provider     string `json:"provider"`
	Sender       string `json:"sender"`
	TextFragment string `json:"text_fragment"`
}

// StreamMessages handles GET /portal/v1/ws/messages (WebSocket upgrade)
func (h *WsMessagesHandlers) StreamMessages(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn().Err(err).Msg("ws_messages: upgrade failed")
		return
	}
	defer conn.Close()

	// Clear HTTP server write deadline — WebSocket connections are long-lived
	// and the net/http WriteTimeout would otherwise kill them.
	conn.SetWriteDeadline(time.Time{}) //nolint:errcheck

	ctx := r.Context()

	// Subscribe to SSE hub for status updates
	// statusCh is nil when hub is unavailable; nil channel in select is a no-op.
	var statusCh chan sse.Event
	if h.hub != nil {
		statusCh = h.hub.Subscribe(clientID.String())
		defer h.hub.Unsubscribe(clientID.String(), statusCh)
	}

	// Poll DB every 2s for recent messages
	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()

	// Ping to keep connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	lastPoll := time.Now().Add(-5 * time.Second)

	send := func(evt wsMessageEvent) bool {
		data, _ := json.Marshal(evt)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Debug().Err(err).Msg("ws_messages: write failed, closing")
			return false
		}
		return true
	}

	// Read loop (detects client disconnect)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return

		case evt, ok := <-statusCh:
			if !ok {
				return
			}
			wsEvt := wsMessageEvent{
				MessageID: evt.MessageID,
				Timestamp: evt.UpdatedAt.Format(time.RFC3339),
				Status:    evt.Status,
			}
			if !send(wsEvt) {
				return
			}

		case <-pollTicker.C:
			if h.dbPool == nil {
				continue
			}
			since := lastPoll
			lastPoll = time.Now()

			rows, err := h.dbPool.Query(ctx,
				`SELECT m.id, m.created_at, m.status, m.destination,
				        COALESCE(m.text, ''), COALESCE(m.source, '')
				 FROM messages m
				 WHERE m.client_id = $1
				   AND m.created_at > $2
				 ORDER BY m.created_at DESC
				 LIMIT 20`,
				clientID, since,
			)
			if err != nil {
				log.Warn().Err(err).Msg("ws_messages: db poll failed")
				continue
			}
			func() {
				defer rows.Close()
				for rows.Next() {
					var (
						id, status, destination, text, source string
						createdAt                             time.Time
					)
					if err := rows.Scan(&id, &createdAt, &status, &destination, &text, &source); err != nil {
						continue
					}
					// TODO: add JOINs to operators/providers tables to populate Operator and Provider fields
					wsEvt := wsMessageEvent{
						MessageID:    id,
						Timestamp:    createdAt.Format(time.RFC3339),
						Status:       status,
						PhoneMasked:  maskPhone(destination),
						Sender:       source,
						TextFragment: truncate(text, 40),
					}
					if !send(wsEvt) {
						return
					}
				}
			}()

		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// maskPhone masks a phone number: replaces middle digits with *
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	runes := []rune(phone)
	for i := 4; i < len(runes) && i < 10; i++ {
		if runes[i] >= '0' && runes[i] <= '9' {
			runes[i] = '*'
		}
	}
	return string(runes)
}

// truncate returns first n characters of s.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
