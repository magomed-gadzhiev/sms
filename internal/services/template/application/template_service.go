package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type TemplateService struct {
	templateRepo TemplateRepository
	auditRepo    AuditRepository
	logger       zerolog.Logger
}

func NewTemplateService(templateRepo TemplateRepository, auditRepo AuditRepository) *TemplateService {
	return &TemplateService{
		templateRepo: templateRepo,
		auditRepo:    auditRepo,
		logger:       log.With().Str("component", "template-service").Logger(),
	}
}

func (s *TemplateService) CreateTemplate(ctx context.Context, clientID uuid.UUID, name, body string) (*domain.Template, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateBody(body); err != nil {
		return nil, err
	}

	variables := domain.ExtractVariables(body)

	tmpl := &domain.Template{
		ID:        uuid.New(),
		ClientID:  clientID,
		Name:      name,
		Body:      body,
		Variables: variables,
		Status:    domain.StatusDraft,
	}

	created, err := s.templateRepo.Create(ctx, tmpl)
	if err != nil {
		return nil, err
	}

	// Audit log
	templateID := created.ID
	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &templateID,
		Action:     "created",
		NewBody:    body,
		ActorType:  "client",
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", templateID.String()).Msg("failed to write audit log")
	}

	return created, nil
}

func (s *TemplateService) GetTemplate(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error) {
	return s.templateRepo.GetByID(ctx, id, clientID)
}

func (s *TemplateService) ListTemplates(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.templateRepo.ListByClientID(ctx, clientID, status, limit, offset)
}

func (s *TemplateService) UpdateTemplate(ctx context.Context, id, clientID uuid.UUID, name, body *string) (*domain.Template, error) {
	existing, err := s.templateRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	oldBody := existing.Body
	bodyChanged := false

	if name != nil {
		if err := validateName(*name); err != nil {
			return nil, err
		}
		existing.Name = *name
	}
	if body != nil {
		if err := validateBody(*body); err != nil {
			return nil, err
		}
		existing.Body = *body
		existing.Variables = domain.ExtractVariables(*body)
		bodyChanged = true
		// Reset status to draft when body changes
		existing.Status = domain.StatusDraft
		existing.RejectionReason = ""
	}

	updated, err := s.templateRepo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	// Audit log
	entry := &domain.AuditEntry{
		TemplateID: &id,
		Action:     "updated",
		ActorType:  "client",
	}
	if bodyChanged {
		entry.OldBody = oldBody
		entry.NewBody = existing.Body
	}
	if err := s.auditRepo.Create(ctx, entry); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return updated, nil
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, id, clientID uuid.UUID) error {
	// Write audit entry before delete (FK becomes NULL via ON DELETE SET NULL)
	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &id,
		Action:     "deleted",
		ActorType:  "client",
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return s.templateRepo.Delete(ctx, id, clientID)
}

func (s *TemplateService) ApproveTemplate(ctx context.Context, id uuid.UUID, actorID *uuid.UUID) (*domain.Template, error) {
	tmpl, err := s.templateRepo.GetByIDAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl.Status != domain.StatusDraft {
		return nil, fmt.Errorf("%w: can only approve draft templates", domain.ErrInvalidStatus)
	}

	updated, err := s.templateRepo.UpdateStatus(ctx, id, domain.StatusApproved, "")
	if err != nil {
		return nil, err
	}

	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &id,
		Action:     "approved",
		ActorID:    actorID,
		ActorType:  "admin",
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return updated, nil
}

func (s *TemplateService) RejectTemplate(ctx context.Context, id uuid.UUID, actorID *uuid.UUID, reason string) (*domain.Template, error) {
	tmpl, err := s.templateRepo.GetByIDAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl.Status != domain.StatusDraft {
		return nil, fmt.Errorf("%w: can only reject draft templates", domain.ErrInvalidStatus)
	}

	updated, err := s.templateRepo.UpdateStatus(ctx, id, domain.StatusRejected, reason)
	if err != nil {
		return nil, err
	}

	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &id,
		Action:     "rejected",
		ActorID:    actorID,
		ActorType:  "admin",
		Reason:     reason,
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return updated, nil
}

func (s *TemplateService) RenderTemplate(ctx context.Context, templateID, clientID uuid.UUID, variables map[string]string) (string, string, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, templateID, clientID)
	if err != nil {
		return "", "", err
	}

	if tmpl.Status != domain.StatusApproved {
		return "", "", domain.ErrTemplateNotApproved
	}

	// Check all required variables are provided
	var missing []string
	for _, v := range tmpl.Variables {
		if _, ok := variables[v]; !ok {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return "", "", fmt.Errorf("%w: %s", domain.ErrMissingVariables, strings.Join(missing, ", "))
	}

	// Validate variable value lengths
	for k, v := range variables {
		if len(v) > domain.MaxVariableValueLength {
			return "", "", fmt.Errorf("%w: variable '%s' is %d chars (max %d)", domain.ErrVariableValueTooLong, k, len(v), domain.MaxVariableValueLength)
		}
	}

	// Render: replace {{var}} with value
	replacerArgs := make([]string, 0, len(variables)*2)
	for k, v := range variables {
		replacerArgs = append(replacerArgs, "{{"+k+"}}", v)
	}
	rendered := strings.NewReplacer(replacerArgs...).Replace(tmpl.Body)

	// Validate rendered length
	if len(rendered) > domain.MaxRenderedLength {
		return "", "", fmt.Errorf("%w: %d chars (max %d)", domain.ErrRenderedTooLong, len(rendered), domain.MaxRenderedLength)
	}

	return rendered, tmpl.Name, nil
}

func (s *TemplateService) GetAuditLog(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.auditRepo.ListByTemplateID(ctx, templateID, limit, offset)
}

func validateName(name string) error {
	if name == "" || len(name) > domain.MaxNameLength {
		return fmt.Errorf("%w: must be 1-%d characters", domain.ErrInvalidTemplateName, domain.MaxNameLength)
	}
	return nil
}

func validateBody(body string) error {
	if body == "" || len(body) > domain.MaxBodyLength {
		return fmt.Errorf("%w: must be 1-%d characters", domain.ErrInvalidTemplateBody, domain.MaxBodyLength)
	}
	return nil
}
