package application

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
)

const (
	importBatchSize = 1000
	uploadsDir      = "uploads"
)

// ImportWorker processes CSV import jobs.
type ImportWorker struct {
	contactRepo *repository.ContactRepository
	importRepo  *repository.ImportRepository
	listRepo    *repository.ContactListRepository
	logger      zerolog.Logger
}

// NewImportWorker creates a new ImportWorker.
func NewImportWorker(
	contactRepo *repository.ContactRepository,
	importRepo *repository.ImportRepository,
	listRepo *repository.ContactListRepository,
	logger zerolog.Logger,
) *ImportWorker {
	return &ImportWorker{
		contactRepo: contactRepo,
		importRepo:  importRepo,
		listRepo:    listRepo,
		logger:      logger.With().Str("component", "import-worker").Logger(),
	}
}

// Process runs the import job. Called as a goroutine.
func (w *ImportWorker) Process(job *domain.ImportJob) {
	ctx := context.Background()

	w.logger.Info().
		Str("import_id", job.ID.String()).
		Str("file_name", job.FileName).
		Msg("starting import processing")

	// Update status to processing
	job.Status = domain.ImportStatusProcessing
	if err := w.importRepo.UpdateStatus(ctx, job); err != nil {
		w.logger.Error().Err(err).Str("import_id", job.ID.String()).Msg("failed to update import status to processing")
		return
	}

	// Open CSV file
	filePath := filepath.Join(uploadsDir, job.ClientID.String(), job.ID.String(), job.FileName)
	file, err := os.Open(filePath)
	if err != nil {
		w.failImport(ctx, job, fmt.Sprintf("failed to open file: %v", err))
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		w.failImport(ctx, job, fmt.Sprintf("failed to read CSV header: %v", err))
		return
	}
	_ = header // We use column_mapping to determine field positions

	// Build column mapping index
	mapping := job.ColumnMapping
	if len(mapping) == 0 {
		// Auto-detect: try to find phone column from header
		mapping = w.autoDetectMapping(header)
	}

	// Find phone column index
	phoneColIdx := -1
	for _, m := range mapping {
		if m.Target.Type == "phone" {
			phoneColIdx = m.Column
			break
		}
	}
	if phoneColIdx == -1 {
		w.failImport(ctx, job, "no phone column mapped")
		return
	}

	// Process rows in batches
	var batch []domain.Contact
	rowNum := 1 // 1-indexed, after header
	totalRows := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		totalRows++

		if err != nil {
			job.ErrorCount++
			job.Errors = append(job.Errors, domain.ImportError{
				Row:     rowNum,
				Message: fmt.Sprintf("CSV parse error: %v", err),
			})
			continue
		}

		// Extract phone
		if phoneColIdx >= len(record) {
			job.ErrorCount++
			job.Errors = append(job.Errors, domain.ImportError{
				Row:     rowNum,
				Column:  "phone",
				Message: "phone column index out of range",
			})
			continue
		}

		phone := normalizePhone(record[phoneColIdx])
		if !phoneRegex.MatchString(phone) {
			job.ErrorCount++
			job.Errors = append(job.Errors, domain.ImportError{
				Row:     rowNum,
				Column:  "phone",
				Message: fmt.Sprintf("invalid phone: %s", record[phoneColIdx]),
			})
			continue
		}

		// Build attributes and tags from mapping
		attrs := make(map[string]interface{})
		var tags []string

		for _, m := range mapping {
			if m.Target.Type == "phone" {
				continue // already handled
			}
			if m.Column >= len(record) {
				continue
			}
			value := strings.TrimSpace(record[m.Column])
			if value == "" {
				continue
			}

			switch m.Target.Type {
			case "attribute":
				attrs[m.Target.Field] = value
			case "tag":
				// Split by comma if value contains commas, else single tag
				for _, t := range strings.Split(value, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
			}
		}

		contact := domain.Contact{
			ID:            uuid.New(),
			ContactListID: job.ContactListID,
			Phone:         phone,
			Attributes:    attrs,
			Tags:          tags,
		}
		batch = append(batch, contact)

		// Flush batch when full
		if len(batch) >= importBatchSize {
			w.flushBatch(ctx, job, batch)
			batch = batch[:0]
		}
	}

	// Flush remaining
	if len(batch) > 0 {
		w.flushBatch(ctx, job, batch)
	}

	// Mark completed
	job.TotalRows = int32(totalRows)
	job.Status = domain.ImportStatusCompleted
	now := time.Now()
	job.CompletedAt = &now

	// Cap errors list to prevent huge JSONB
	if len(job.Errors) > 100 {
		job.Errors = job.Errors[:100]
	}

	if err := w.importRepo.UpdateStatus(ctx, job); err != nil {
		w.logger.Error().Err(err).Str("import_id", job.ID.String()).Msg("failed to update import status to completed")
	}

	// Update contacts count
	if err := w.listRepo.UpdateContactsCount(ctx, job.ContactListID); err != nil {
		w.logger.Error().Err(err).Str("list_id", job.ContactListID.String()).Msg("failed to update contacts count after import")
	}

	w.logger.Info().
		Str("import_id", job.ID.String()).
		Int32("imported", job.ImportedCount).
		Int32("updated", job.UpdatedCount).
		Int32("errors", job.ErrorCount).
		Msg("import completed")
}

// flushBatch upserts a batch of contacts and updates import counters.
func (w *ImportWorker) flushBatch(ctx context.Context, job *domain.ImportJob, batch []domain.Contact) {
	created, updated, err := w.contactRepo.BatchUpsert(ctx, job.ContactListID, batch)
	if err != nil {
		w.logger.Error().Err(err).
			Str("import_id", job.ID.String()).
			Int("batch_size", len(batch)).
			Msg("batch upsert failed")
		job.ErrorCount += int32(len(batch))
		return
	}

	job.ImportedCount += created
	job.UpdatedCount += updated

	// Periodic status update for progress tracking
	if err := w.importRepo.UpdateStatus(ctx, job); err != nil {
		w.logger.Error().Err(err).Str("import_id", job.ID.String()).Msg("failed to update import progress")
	}
}

// failImport marks the import as failed.
func (w *ImportWorker) failImport(ctx context.Context, job *domain.ImportJob, message string) {
	w.logger.Error().Str("import_id", job.ID.String()).Msg(message)
	job.Status = domain.ImportStatusFailed
	now := time.Now()
	job.CompletedAt = &now
	job.Errors = append(job.Errors, domain.ImportError{
		Row:     0,
		Message: message,
	})
	if err := w.importRepo.UpdateStatus(ctx, job); err != nil {
		w.logger.Error().Err(err).Str("import_id", job.ID.String()).Msg("failed to update import status to failed")
	}
}

// autoDetectMapping tries to auto-detect column mapping from CSV header.
func (w *ImportWorker) autoDetectMapping(header []string) []domain.ColumnMapping {
	var mapping []domain.ColumnMapping
	for i, col := range header {
		col = strings.TrimSpace(strings.ToLower(col))
		switch {
		case col == "phone" || col == "msisdn" || col == "number" || col == "mobile":
			mapping = append(mapping, domain.ColumnMapping{
				Column: i,
				Target: domain.ColumnTarget{Field: "phone", Type: "phone"},
			})
		case col == "tag" || col == "tags":
			mapping = append(mapping, domain.ColumnMapping{
				Column: i,
				Target: domain.ColumnTarget{Field: col, Type: "tag"},
			})
		default:
			mapping = append(mapping, domain.ColumnMapping{
				Column: i,
				Target: domain.ColumnTarget{Field: col, Type: "attribute"},
			})
		}
	}
	return mapping
}
