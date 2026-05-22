package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
)

// ReportService предоставляет бизнес-логику для генерации отчетов
type ReportService struct {
	reportRepo  domain.ReportRepository
	metricRepo  domain.MetricRepository
}

// NewReportService создает новый сервис отчетов
func NewReportService(reportRepo domain.ReportRepository, metricRepo domain.MetricRepository) *ReportService {
	return &ReportService{
		reportRepo: reportRepo,
		metricRepo: metricRepo,
	}
}

// GenerateReport генерирует отчет указанного типа
func (s *ReportService) GenerateReport(
	ctx context.Context,
	reportType domain.ReportType,
	format domain.ReportFormat,
	from, to time.Time,
	clientID *uuid.UUID,
	providerID *uuid.UUID,
) (*domain.Report, error) {
	// Получаем статистику для периода
	filters := &domain.StatisticsFilters{
		From:     from,
		To:       to,
		ClientID: clientID,
	}

	if providerID != nil {
		filters.ProviderIDs = []uuid.UUID{*providerID}
	}

	stats, err := s.metricRepo.GetStatistics(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	// Генерируем данные отчета в зависимости от формата
	var data []byte
	var genErr error

	switch format {
	case domain.ReportFormatJSON:
		data, genErr = s.generateJSONReport(stats, reportType, from, to, clientID, providerID)
	case domain.ReportFormatCSV:
		data, genErr = s.generateCSVReport(stats, reportType, from, to, clientID, providerID)
	case domain.ReportFormatPDF:
		// PDF генерация требует дополнительных библиотек
		// Пока возвращаем JSON в качестве fallback
		data, genErr = s.generateJSONReport(stats, reportType, from, to, clientID, providerID)
	default:
		return nil, fmt.Errorf("unsupported report format: %s", format)
	}

	if genErr != nil {
		return nil, fmt.Errorf("failed to generate report data: %w", genErr)
	}

	// Создаем отчет
	report := domain.NewReport(reportType, format, from, to, clientID, providerID, data)

	// Сохраняем отчет
	if err := s.reportRepo.Create(ctx, report); err != nil {
		log.Error().Err(err).Msg("ошибка сохранения отчета")
		return nil, fmt.Errorf("failed to save report: %w", err)
	}

	return report, nil
}

// GetReport получает отчет по ID
func (s *ReportService) GetReport(ctx context.Context, reportID uuid.UUID) (*domain.Report, error) {
	return s.reportRepo.GetByID(ctx, reportID)
}

// ListReports получает список отчетов
func (s *ReportService) ListReports(ctx context.Context, filters *domain.ReportFilters) ([]*domain.Report, error) {
	return s.reportRepo.List(ctx, filters)
}

// generateJSONReport генерирует JSON отчет
func (s *ReportService) generateJSONReport(
	stats *domain.Statistics,
	reportType domain.ReportType,
	from, to time.Time,
	clientID *uuid.UUID,
	providerID *uuid.UUID,
) ([]byte, error) {
	reportData := map[string]interface{}{
		"type":        reportType,
		"period_start": from,
		"period_end":   to,
		"statistics":   stats,
	}

	if clientID != nil {
		reportData["client_id"] = clientID.String()
	}

	if providerID != nil {
		reportData["provider_id"] = providerID.String()
	}

	return json.Marshal(reportData)
}

// generateCSVReport генерирует CSV отчет
func (s *ReportService) generateCSVReport(
	stats *domain.Statistics,
	reportType domain.ReportType,
	from, to time.Time,
	clientID *uuid.UUID,
	providerID *uuid.UUID,
) ([]byte, error) {
	// Простая CSV генерация
	csv := "Period Start,Period End,Total Sent,Total Delivered,Total Failed,Success Rate\n"
	csv += fmt.Sprintf("%s,%s,%d,%d,%d,%d\n",
		from.Format(time.RFC3339),
		to.Format(time.RFC3339),
		stats.Totals.TotalSent,
		stats.Totals.TotalDelivered,
		stats.Totals.TotalFailed,
		stats.Totals.SuccessRate,
	)

	// Добавляем группы
	if len(stats.Groups) > 0 {
		csv += "\nGroup,Total Sent,Total Delivered,Total Failed\n"
		for _, group := range stats.Groups {
			csv += fmt.Sprintf("%s,%d,%d,%d\n",
				group.Key,
				group.Stats.TotalSent,
				group.Stats.TotalDelivered,
				group.Stats.TotalFailed,
			)
		}
	}

	return []byte(csv), nil
}
