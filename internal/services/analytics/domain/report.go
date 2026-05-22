package domain

import (
	"time"

	"github.com/google/uuid"
)

// Report представляет доменную модель отчета
type Report struct {
	ID          uuid.UUID
	Type        ReportType
	Format      ReportFormat
	ClientID    *uuid.UUID
	ProviderID  *uuid.UUID
	PeriodStart time.Time
	PeriodEnd   time.Time
	Data        []byte
	GeneratedAt time.Time
	CreatedAt   time.Time
}

// ReportType представляет тип отчета
type ReportType string

const (
	ReportTypeDaily    ReportType = "daily"
	ReportTypeMonthly  ReportType = "monthly"
	ReportTypeProvider ReportType = "provider"
	ReportTypeClient   ReportType = "client"
)

// ReportFormat представляет формат отчета
type ReportFormat string

const (
	ReportFormatJSON ReportFormat = "json"
	ReportFormatCSV  ReportFormat = "csv"
	ReportFormatPDF  ReportFormat = "pdf"
)

// NewReport создает новый отчет
func NewReport(
	reportType ReportType,
	format ReportFormat,
	periodStart time.Time,
	periodEnd time.Time,
	clientID *uuid.UUID,
	providerID *uuid.UUID,
	data []byte,
) *Report {
	now := time.Now()
	return &Report{
		ID:          uuid.New(),
		Type:        reportType,
		Format:      format,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		ClientID:    clientID,
		ProviderID:  providerID,
		Data:        data,
		GeneratedAt: now,
		CreatedAt:   now,
	}
}
