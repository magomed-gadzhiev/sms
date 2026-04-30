package domain

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTemplateNotFound      = errors.New("template not found")
	ErrTemplateNotApproved   = errors.New("template is not approved")
	ErrInvalidTemplateName   = errors.New("invalid template name")
	ErrInvalidTemplateBody   = errors.New("invalid template body")
	ErrMissingVariables      = errors.New("missing required variables")
	ErrRenderedTooLong       = errors.New("rendered text exceeds maximum length")
	ErrVariableValueTooLong  = errors.New("variable value exceeds maximum length")
	ErrInvalidStatus         = errors.New("invalid template status for this operation")
	ErrDuplicateTemplateName = errors.New("template with this name already exists")
	ErrInvalidTrafficType    = errors.New("invalid traffic_type")
)

// AllowedTrafficTypes — допустимые значения traffic_type (BUG-63).
// До фикса любая строка принималась и сохранялась в БД, нарушая enum-семантику
// и потенциально ломая фильтрацию по типу трафика в тарификации.
// Список синхронизирован с routing/domain.ValidTrafficType
// (internal/services/routing/domain/route_types.go), pipeline default
// "transactional" и frontend ConditionEditor.tsx — три значения.
// Если когда-нибудь нужно добавить marketing/extensible — обновить ОБА enum'а.
var AllowedTrafficTypes = map[string]struct{}{
	"authorization": {},
	"transactional": {},
	"service":       {},
}

const (
	StatusDraft             = "draft"
	StatusPending           = "pending"
	StatusApproved          = "approved"
	StatusRejected          = "rejected"
	StatusReview            = "review"
	StatusRevisionRequested = "revision_requested"

	MaxBodyLength          = 1600
	MaxNameLength          = 255
	MaxVariableValueLength = 500
	MaxRenderedLength      = 1600
)

var variableRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)

// Template represents a message template
type Template struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	Name            string
	Body            string
	Variables       []string
	Status          string
	RejectionReason string
	ReviewerID      *uuid.UUID
	ReviewComment   string
	ReviewedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	SenderNameID    *uuid.UUID
	SenderName      string // denormalized for display
	TrafficType     string // authorization, transactional (default), service
}

// AuditEntry represents a template audit log entry
type AuditEntry struct {
	ID         uuid.UUID
	TemplateID *uuid.UUID
	Action     string
	OldBody    string
	NewBody    string
	ActorID    *uuid.UUID
	ActorType  string
	Reason     string
	CreatedAt  time.Time
}

// ExtractVariables parses {{variable}} placeholders from template body
func ExtractVariables(body string) []string {
	matches := variableRegex.FindAllStringSubmatch(body, -1)
	seen := make(map[string]bool)
	vars := make([]string, 0)
	for _, match := range matches {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}
