package application

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// ExportWorker processes pending export jobs in the background.
type ExportWorker struct {
	service    *NetworkAnalyticsService
	exportRepo domain.ExportRepository
	exportDir  string
	log        zerolog.Logger
}

// NewExportWorker creates a new ExportWorker.
func NewExportWorker(service *NetworkAnalyticsService, exportRepo domain.ExportRepository, exportDir string, log zerolog.Logger) *ExportWorker {
	return &ExportWorker{
		service:    service,
		exportRepo: exportRepo,
		exportDir:  exportDir,
		log:        log,
	}
}

// Run polls for pending export jobs and processes them until ctx is cancelled.
func (w *ExportWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processPending(ctx)
		}
	}
}

func (w *ExportWorker) processPending(ctx context.Context) {
	jobs, err := w.exportRepo.GetPendingJobs(ctx, 5)
	if err != nil {
		w.log.Error().Err(err).Msg("export worker: get pending jobs")
		return
	}
	for _, job := range jobs {
		w.processJob(ctx, job)
	}
}

func (w *ExportWorker) processJob(ctx context.Context, job domain.ExportJob) {
	now := time.Now().UTC()
	job.Status = "processing"
	_ = w.exportRepo.UpdateJob(ctx, &job)

	filePath, rowCount, err := w.generateFile(ctx, &job)
	now = time.Now().UTC()
	if err != nil {
		w.log.Error().Err(err).Str("job_id", job.ID).Msg("export worker: generate file")
		job.Status = "failed"
		job.Error = err.Error()
		job.CompletedAt = &now
		_ = w.exportRepo.UpdateJob(ctx, &job)
		return
	}

	job.Status = "done"
	job.FilePath = filePath
	job.RowCount = rowCount
	job.CompletedAt = &now
	if err := w.exportRepo.UpdateJob(ctx, &job); err != nil {
		w.log.Error().Err(err).Str("job_id", job.ID).Msg("export worker: update job")
	}
	w.log.Info().Str("job_id", job.ID).Str("path", filePath).Int("rows", rowCount).Msg("export worker: job done")
}

// filterFromJSON decodes the stored proto-JSON filter into a domain.SharedFilter.
type exportFilterJSON struct {
	PeriodPreset  string `json:"period_preset"`
	DateFrom      int64  `json:"date_from"`
	DateTo        int64  `json:"date_to"`
	GroupBy       string `json:"group_by"`
	Login         string `json:"login"`
	ServiceType   string `json:"service_type"`
	Operator      string `json:"operator"`
	Channel       string `json:"channel"`
	SenderName    string `json:"sender_name"`
	SenderPaid    string `json:"sender_paid"`
	International bool   `json:"international"`
	TrafficType   string `json:"traffic_type"`
	Status        string `json:"status"`
	PriceRange    string `json:"price_range"`
	Method        string `json:"method"`
	Provider      string `json:"provider"`
	Country       string `json:"country"`
	Manager       string `json:"manager"`
	ErrorCode     string `json:"error_code"`
	PartnerID     int64  `json:"partner_id"`
}

func filterFromJSON(raw string, partnerID int64) *domain.SharedFilter {
	var j exportFilterJSON
	_ = json.Unmarshal([]byte(raw), &j)
	f := &domain.SharedFilter{
		PartnerID:     partnerID,
		PeriodPreset:  j.PeriodPreset,
		GroupBy:       j.GroupBy,
		Login:         j.Login,
		ServiceType:   j.ServiceType,
		Operator:      j.Operator,
		Channel:       j.Channel,
		SenderName:    j.SenderName,
		SenderPaid:    j.SenderPaid,
		International: j.International,
		TrafficType:   j.TrafficType,
		Status:        j.Status,
		PriceRange:    j.PriceRange,
		Method:        j.Method,
		Provider:      j.Provider,
		Country:       j.Country,
		Manager:       j.Manager,
		ErrorCode:     j.ErrorCode,
		Page:          1,
		PageSize:      10000,
	}
	if j.DateFrom != 0 {
		f.DateFrom = time.Unix(j.DateFrom, 0).UTC()
	}
	if j.DateTo != 0 {
		f.DateTo = time.Unix(j.DateTo, 0).UTC()
	}
	return f
}

func (w *ExportWorker) generateFile(ctx context.Context, job *domain.ExportJob) (string, int, error) {
	if err := os.MkdirAll(w.exportDir, 0755); err != nil {
		return "", 0, fmt.Errorf("mkdir exports: %w", err)
	}

	ext := "csv"
	if job.Format == "xlsx" {
		ext = "csv" // generate CSV even for xlsx requests (xlsx requires external lib)
	}
	filePath := filepath.Join(w.exportDir, fmt.Sprintf("%s.%s", job.ID, ext))

	f, err := os.Create(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	filter := filterFromJSON(job.Filters, job.PartnerID)
	filter.Normalize()

	w2 := csv.NewWriter(f)

	var rowCount int
	switch job.Mode {
	case "monitoring":
		rowCount, err = w.writeMonitoringCSV(ctx, w2, filter)
	default:
		rowCount, err = w.writeStatisticsCSV(ctx, w2, filter)
	}
	if err != nil {
		return "", 0, err
	}
	w2.Flush()
	return filePath, rowCount, w2.Error()
}

func (w *ExportWorker) writeStatisticsCSV(ctx context.Context, cw *csv.Writer, filter *domain.SharedFilter) (int, error) {
	_ = cw.Write([]string{
		"Срез", "Всего", "Отправлено", "Доставлено", "Ошибки", "Ожидание", "Тайм-аут",
		"DLR %", "Выручка", "Стоимость", "Прибыль", "Маржа %",
	})

	rows, _, err := w.service.statsRepo.GetStatistics(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("get statistics: %w", err)
	}
	for _, r := range rows {
		_ = cw.Write([]string{
			r.Slice,
			strconv.FormatInt(r.Total, 10),
			strconv.FormatInt(r.Sent, 10),
			strconv.FormatInt(r.Delivered, 10),
			strconv.FormatInt(r.Failed, 10),
			strconv.FormatInt(r.Pending, 10),
			strconv.FormatInt(r.Timeout, 10),
			fmt.Sprintf("%.2f", r.DLRRate*100),
			fmt.Sprintf("%.4f", r.Revenue),
			fmt.Sprintf("%.4f", r.Cost),
			fmt.Sprintf("%.4f", r.Profit),
			fmt.Sprintf("%.2f", r.Margin*100),
		})
	}
	return len(rows), nil
}

func (w *ExportWorker) writeMonitoringCSV(ctx context.Context, cw *csv.Writer, filter *domain.SharedFilter) (int, error) {
	_ = cw.Write([]string{
		"Срез", "Пропускная способность", "Отправлено", "Доставлено", "Ожидание",
		"Тайм-аут", "Ошибки", "Задержка P50 (мс)", "Задержка P95 (мс)", "DLR %", "Топ ошибка",
	})

	rows, err := w.service.monitoringRepo.GetMonitoringMetrics(ctx, filter, false)
	if err != nil {
		return 0, fmt.Errorf("get monitoring: %w", err)
	}
	for _, r := range rows {
		_ = cw.Write([]string{
			r.Slice,
			fmt.Sprintf("%.2f", r.Throughput),
			strconv.FormatInt(r.Sent, 10),
			strconv.FormatInt(r.Delivered, 10),
			strconv.FormatInt(r.Pending, 10),
			strconv.FormatInt(r.Timeout, 10),
			strconv.FormatInt(r.Error, 10),
			fmt.Sprintf("%.0f", r.DLRLatencyP50),
			fmt.Sprintf("%.0f", r.DLRLatencyP95),
			fmt.Sprintf("%.2f", r.DLRRate*100),
			r.TopError,
		})
	}
	return len(rows), nil
}
