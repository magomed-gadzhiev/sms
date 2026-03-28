package redirect

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/link/application"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

type Handler struct {
	service *application.LinkService
	logger  zerolog.Logger
}

func NewHandler(service *application.LinkService) *Handler {
	return &Handler{
		service: service,
		logger:  log.With().Str("component", "redirect-handler").Logger(),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/")
	if code == "" || strings.Contains(code, "/") {
		http.NotFound(w, r)
		return
	}

	if h.service == nil {
		http.NotFound(w, r)
		return
	}

	link, err := h.service.ResolveCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Record click asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		event := &domain.ClickEvent{
			ID:          uuid.New(),
			ShortLinkID: link.ID,
			ClientID:    link.ClientID,
			CampaignID:  link.CampaignID,
			ClickedAt:   time.Now(),
			IPAddress:   ip,
			UserAgent:   r.UserAgent(),
			Referer:     r.Referer(),
		}
		if err := h.service.RecordClick(ctx, event); err != nil {
			h.logger.Error().Err(err).Str("code", code).Msg("failed to record click")
		}
	}()

	http.Redirect(w, r, link.OriginalURL, http.StatusFound)
}

// StartRedirectServer starts the lightweight HTTP redirect server.
func StartRedirectServer(ctx context.Context, port int, service *application.LinkService) error {
	handler := NewHandler(service)
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServe()
}
