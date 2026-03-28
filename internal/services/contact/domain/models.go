package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrContactListNotFound = errors.New("contact list not found")
	ErrContactNotFound     = errors.New("contact not found")
	ErrImportNotFound      = errors.New("import not found")
	ErrDuplicatePhone      = errors.New("duplicate phone number in contact list")
	ErrInvalidPhone        = errors.New("invalid phone number")
	ErrInvalidSegmentRules = errors.New("invalid segment rules")
	ErrSegmentDepthExceeded = errors.New("segment rule nesting depth exceeded (max 3)")
	ErrImportAlreadyStarted = errors.New("import is already in progress")
)

// Import statuses
const (
	ImportStatusPending    = "pending"
	ImportStatusProcessing = "processing"
	ImportStatusCompleted  = "completed"
	ImportStatusFailed     = "failed"
)

// Attribute types
const (
	AttrTypeString  = "string"
	AttrTypeNumber  = "number"
	AttrTypeDate    = "date"
	AttrTypeBoolean = "boolean"
)

// ContactList represents a contact list owned by a client
type ContactList struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	Name          string
	Description   string
	ContactsCount int32
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Contact represents a single contact within a list
type Contact struct {
	ID            uuid.UUID
	ContactListID uuid.UUID
	Phone         string
	Attributes    map[string]interface{}
	Tags          []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ContactAttribute defines a custom attribute schema for a contact list
type ContactAttribute struct {
	ID            uuid.UUID
	ContactListID uuid.UUID
	Name          string
	DisplayName   string
	Type          string
	Required      bool
	Position      int32
}

// ImportJob represents a CSV import job
type ImportJob struct {
	ID            uuid.UUID
	ContactListID uuid.UUID
	ClientID      uuid.UUID
	FileName      string
	FileSize      int64
	Status        string
	TotalRows     int32
	ImportedCount int32
	UpdatedCount  int32
	ErrorCount    int32
	Errors        []ImportError
	ColumnMapping []ColumnMapping
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// ImportError represents a single error during import
type ImportError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// ColumnMapping defines how a CSV column maps to a contact field
type ColumnMapping struct {
	Column int          `json:"column"`
	Target ColumnTarget `json:"target"`
}

// ColumnTarget specifies what a CSV column maps to
type ColumnTarget struct {
	Field string `json:"field"` // "phone", "tag", or attribute name
	Type  string `json:"type"`  // "phone", "tag", "attribute"
}

// BatchUpsertResult holds results of a batch upsert operation
type BatchUpsertResult struct {
	Created int32
	Updated int32
	Errors  int32
	ErrorMessages []string
}
