package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"

	"github.com/google/uuid"
)

var exportJobsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "portal_export_jobs_total",
	Help: "Total number of portal CSV export jobs started",
})

const exportTTL = time.Hour

// ExportHandlers содержит handlers для экспорта данных в CSV
type ExportHandlers struct {
	redisClient     *redis.Client
	messagingClient messagingv1.MessagingServiceClient
}

// NewExportHandlers создает ExportHandlers
func NewExportHandlers(redisClient *redis.Client, messagingClient messagingv1.MessagingServiceClient) *ExportHandlers {
	return &ExportHandlers{redisClient: redisClient, messagingClient: messagingClient}
}

type startExportRequest struct {
	Status      string `json:"status"`
	DateFrom    string `json:"date_from"`
	DateTo      string `json:"date_to"`
	Destination string `json:"destination"`
}

// StartExport обрабатывает POST /export/start
func (h *ExportHandlers) StartExport(w http.ResponseWriter, r *http.Request) {
	log.Debug().Msg("запуск экспорта CSV")

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	var req startExportRequest
	json.NewDecoder(r.Body).Decode(&req) // optional body — ignore decode error

	jobID := uuid.New().String()
	key := fmt.Sprintf("export:job:%s", jobID)

	ctx := r.Context()

	err := h.redisClient.HSet(ctx, key,
		"status", "pending",
		"user_id", userID.String(),
		"client_id", clientID.String(),
	).Err()
	if err != nil {
		log.Error().Err(err).Msg("ошибка записи задачи экспорта в Redis")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	h.redisClient.Expire(ctx, key, exportTTL)

	exportJobsTotal.Inc()

	go h.runExportJob(jobID, clientID.String(), req)

	respondJSON(w, http.StatusAccepted, map[string]interface{}{"job_id": jobID})
}

func (h *ExportHandlers) runExportJob(jobID, clientID string, req startExportRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	key := fmt.Sprintf("export:job:%s", jobID)
	filePath := fmt.Sprintf("/tmp/export_%s.csv", jobID)

	h.redisClient.HSet(ctx, key, "status", "processing")

	f, err := os.Create(filePath)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("ошибка создания файла экспорта")
		h.redisClient.HSet(ctx, key, "status", "error", "error_message", err.Error())
		return
	}
	defer f.Close()

	cw := csv.NewWriter(f)
	cw.Write([]string{"id", "source", "destination", "text", "status", "segment_count", "created_at", "delivered_at"})

	page := int32(1)
	perPage := int32(1000)
	totalWritten := 0

	for {
		if h.messagingClient == nil {
			break
		}
		params := &messagingv1.GetMessageHistoryRequest{
			ClientId: clientID,
			Limit:    perPage,
			Offset:   (page - 1) * perPage,
		}
		if req.Status != "" {
			params.Status = req.Status
		}
		if req.DateFrom != "" {
			if t, err := time.Parse("2006-01-02", req.DateFrom); err == nil {
				params.From = timestamppb.New(t)
			}
		}
		if req.DateTo != "" {
			if t, err := time.Parse("2006-01-02", req.DateTo); err == nil {
				params.To = timestamppb.New(t.Add(24*time.Hour - time.Second))
			}
		}
		if req.Destination != "" {
			params.Destination = req.Destination
		}

		resp, err := h.messagingClient.GetMessageHistory(ctx, params)
		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("ошибка получения сообщений для экспорта")
			break
		}

		for _, msg := range resp.Messages {
			deliveredAt := ""
			if msg.DeliveredAt != nil {
				deliveredAt = msg.DeliveredAt.AsTime().Format(time.RFC3339)
			}
			createdAt := ""
			if msg.CreatedAt != nil {
				createdAt = msg.CreatedAt.AsTime().Format(time.RFC3339)
			}
			cw.Write([]string{
				msg.MessageId, msg.Source, msg.Destination, msg.Text, msg.Status,
				strconv.Itoa(int(msg.SegmentCount)), createdAt, deliveredAt,
			})
			totalWritten++
		}

		if int32(len(resp.Messages)) < perPage {
			break
		}
		page++
	}

	cw.Flush()

	h.redisClient.HSet(ctx, key,
		"status", "ready",
		"file_path", filePath,
		"total_rows", strconv.Itoa(totalWritten),
	)
	h.redisClient.Expire(ctx, key, exportTTL)

	log.Info().Str("job_id", jobID).Int("rows", totalWritten).Msg("экспорт завершён")
}

// GetExportStatus обрабатывает GET /export/{job_id}/status
func (h *ExportHandlers) GetExportStatus(w http.ResponseWriter, r *http.Request) {
	log.Debug().Msg("статус задачи экспорта")

	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	jobID := mux.Vars(r)["job_id"]
	key := fmt.Sprintf("export:job:%s", jobID)

	ctx := r.Context()
	data, err := h.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		respondError(w, shared.ErrNotFound("Задача экспорта не найдена"))
		return
	}

	if data["user_id"] != userID.String() {
		respondError(w, shared.ErrForbidden("Доступ запрещён"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"job_id":     jobID,
		"status":     data["status"],
		"total_rows": data["total_rows"],
	})
}

// DownloadExport обрабатывает GET /export/{job_id}/download
func (h *ExportHandlers) DownloadExport(w http.ResponseWriter, r *http.Request) {
	log.Debug().Msg("скачивание файла экспорта")

	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	jobID := mux.Vars(r)["job_id"]
	key := fmt.Sprintf("export:job:%s", jobID)

	ctx := r.Context()
	data, err := h.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		respondError(w, shared.ErrNotFound("Задача экспорта не найдена"))
		return
	}

	if data["user_id"] != userID.String() {
		respondError(w, shared.ErrForbidden("Доступ запрещён"))
		return
	}

	switch data["status"] {
	case "pending", "processing":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "EXPORT_NOT_READY",
				"message": "Экспорт ещё не готов",
			},
		})
		return
	case "ready":
		// continue
	default:
		respondError(w, shared.ErrNotFound("Файл экспорта недоступен"))
		return
	}

	filePath := data["file_path"]
	f, err := os.Open(filePath)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("файл экспорта не найден")
		respondError(w, shared.ErrNotFound("Файл экспорта не найден"))
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="export_%s.csv"`, jobID))
	w.WriteHeader(http.StatusOK)

	buf := make([]byte, 32*1024)
	for {
		n, rErr := f.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if rErr != nil {
			break
		}
	}

	os.Remove(filePath)
	h.redisClient.Del(ctx, key)
}
