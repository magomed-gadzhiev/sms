package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReport_AllFieldsPopulated(t *testing.T) {
	clientID := uuid.New()
	providerID := uuid.New()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)
	data := []byte(`{"total": 100}`)

	before := time.Now()
	r := NewReport(ReportTypeMonthly, ReportFormatJSON, start, end, &clientID, &providerID, data)
	after := time.Now()

	require.NotNil(t, r)
	assert.NotEqual(t, uuid.Nil, r.ID)
	assert.Equal(t, ReportTypeMonthly, r.Type)
	assert.Equal(t, ReportFormatJSON, r.Format)
	assert.Equal(t, start, r.PeriodStart)
	assert.Equal(t, end, r.PeriodEnd)
	assert.Equal(t, &clientID, r.ClientID)
	assert.Equal(t, &providerID, r.ProviderID)
	assert.Equal(t, data, r.Data)
	assert.False(t, r.GeneratedAt.Before(before))
	assert.False(t, r.GeneratedAt.After(after))
	assert.False(t, r.CreatedAt.Before(before))
	assert.False(t, r.CreatedAt.After(after))
}

func TestNewReport_NilOptionalFields(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	r := NewReport(ReportTypeDaily, ReportFormatCSV, start, end, nil, nil, nil)

	require.NotNil(t, r)
	assert.Nil(t, r.ClientID)
	assert.Nil(t, r.ProviderID)
	assert.Nil(t, r.Data)
}

func TestNewReport_EmptyData(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	r := NewReport(ReportTypeDaily, ReportFormatJSON, start, end, nil, nil, []byte{})

	require.NotNil(t, r)
	assert.Empty(t, r.Data)
}

func TestNewReport_UniqueIDs(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	r1 := NewReport(ReportTypeDaily, ReportFormatJSON, start, end, nil, nil, nil)
	r2 := NewReport(ReportTypeDaily, ReportFormatJSON, start, end, nil, nil, nil)

	assert.NotEqual(t, r1.ID, r2.ID)
}

func TestNewReport_AllReportTypes(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	types := []ReportType{ReportTypeDaily, ReportTypeMonthly, ReportTypeProvider, ReportTypeClient}
	for _, rt := range types {
		t.Run(string(rt), func(t *testing.T) {
			r := NewReport(rt, ReportFormatJSON, start, end, nil, nil, nil)
			assert.Equal(t, rt, r.Type)
		})
	}
}

func TestNewReport_AllReportFormats(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	formats := []ReportFormat{ReportFormatJSON, ReportFormatCSV, ReportFormatPDF}
	for _, f := range formats {
		t.Run(string(f), func(t *testing.T) {
			r := NewReport(ReportTypeDaily, f, start, end, nil, nil, nil)
			assert.Equal(t, f, r.Format)
		})
	}
}

func TestReportTypeConstants(t *testing.T) {
	assert.Equal(t, ReportType("daily"), ReportTypeDaily)
	assert.Equal(t, ReportType("monthly"), ReportTypeMonthly)
	assert.Equal(t, ReportType("provider"), ReportTypeProvider)
	assert.Equal(t, ReportType("client"), ReportTypeClient)
}

func TestReportFormatConstants(t *testing.T) {
	assert.Equal(t, ReportFormat("json"), ReportFormatJSON)
	assert.Equal(t, ReportFormat("csv"), ReportFormatCSV)
	assert.Equal(t, ReportFormat("pdf"), ReportFormatPDF)
}
