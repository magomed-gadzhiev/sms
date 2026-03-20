package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/services/template/application"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type Server struct {
	templatev1.UnimplementedTemplateServiceServer
	templateService *application.TemplateService
	logger          zerolog.Logger
}

func NewServer(templateService *application.TemplateService) *Server {
	return &Server{
		templateService: templateService,
		logger:          log.With().Str("component", "template-grpc-server").Logger(),
	}
}

func (s *Server) CreateTemplate(ctx context.Context, req *templatev1.CreateTemplateRequest) (*templatev1.CreateTemplateResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Body == "" {
		return nil, status.Error(codes.InvalidArgument, "body is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	tmpl, err := s.templateService.CreateTemplate(ctx, clientID, req.Name, req.Body)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &templatev1.CreateTemplateResponse{
		Template: templateToProto(tmpl),
	}, nil
}

func (s *Server) UpdateTemplate(ctx context.Context, req *templatev1.UpdateTemplateRequest) (*templatev1.UpdateTemplateResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	tmpl, err := s.templateService.UpdateTemplate(ctx, id, clientID, req.Name, req.Body)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &templatev1.UpdateTemplateResponse{
		Template: templateToProto(tmpl),
	}, nil
}

func (s *Server) DeleteTemplate(ctx context.Context, req *templatev1.DeleteTemplateRequest) (*templatev1.DeleteTemplateResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	if err := s.templateService.DeleteTemplate(ctx, id, clientID); err != nil {
		return nil, s.mapError(err)
	}

	return &templatev1.DeleteTemplateResponse{Success: true}, nil
}

func (s *Server) GetTemplate(ctx context.Context, req *templatev1.GetTemplateRequest) (*templatev1.GetTemplateResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	tmpl, err := s.templateService.GetTemplate(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &templatev1.GetTemplateResponse{
		Template: templateToProto(tmpl),
	}, nil
}

func (s *Server) ListTemplates(ctx context.Context, req *templatev1.ListTemplatesRequest) (*templatev1.ListTemplatesResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	templates, total, err := s.templateService.ListTemplates(ctx, clientID, req.Status, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, s.mapError(err)
	}

	protoTemplates := make([]*templatev1.TemplateInfo, len(templates))
	for i, tmpl := range templates {
		protoTemplates[i] = templateToProto(tmpl)
	}

	return &templatev1.ListTemplatesResponse{
		Templates: protoTemplates,
		Total:     int32(total),
		Limit:     req.Limit,
		Offset:    req.Offset,
	}, nil
}

func (s *Server) ApproveTemplate(ctx context.Context, req *templatev1.ApproveTemplateRequest) (*templatev1.ApproveTemplateResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	var actorID *uuid.UUID
	if req.ActorId != "" {
		parsed, err := uuid.Parse(req.ActorId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid actor_id format")
		}
		actorID = &parsed
	}

	tmpl, err := s.templateService.ApproveTemplate(ctx, id, actorID)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &templatev1.ApproveTemplateResponse{
		Template: templateToProto(tmpl),
	}, nil
}

func (s *Server) RejectTemplate(ctx context.Context, req *templatev1.RejectTemplateRequest) (*templatev1.RejectTemplateResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	var actorID *uuid.UUID
	if req.ActorId != "" {
		parsed, err := uuid.Parse(req.ActorId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid actor_id format")
		}
		actorID = &parsed
	}

	tmpl, err := s.templateService.RejectTemplate(ctx, id, actorID, req.Reason)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &templatev1.RejectTemplateResponse{
		Template: templateToProto(tmpl),
	}, nil
}

func (s *Server) RenderTemplate(ctx context.Context, req *templatev1.RenderTemplateRequest) (*templatev1.RenderTemplateResponse, error) {
	if req.TemplateId == "" {
		return nil, status.Error(codes.InvalidArgument, "template_id is required")
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	templateID, err := uuid.Parse(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid template_id format")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	renderedText, templateName, err := s.templateService.RenderTemplate(ctx, templateID, clientID, req.Variables)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &templatev1.RenderTemplateResponse{
		RenderedText: renderedText,
		TemplateName: templateName,
	}, nil
}

func (s *Server) GetTemplateAuditLog(ctx context.Context, req *templatev1.GetTemplateAuditLogRequest) (*templatev1.GetTemplateAuditLogResponse, error) {
	if req.TemplateId == "" {
		return nil, status.Error(codes.InvalidArgument, "template_id is required")
	}
	templateID, err := uuid.Parse(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid template_id format")
	}

	entries, total, err := s.templateService.GetAuditLog(ctx, templateID, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, s.mapError(err)
	}

	protoEntries := make([]*templatev1.AuditEntry, len(entries))
	for i, entry := range entries {
		protoEntries[i] = auditToProto(entry)
	}

	return &templatev1.GetTemplateAuditLogResponse{
		Entries: protoEntries,
		Total:   int32(total),
	}, nil
}

func (s *Server) parseIDs(idStr, clientIDStr string) (uuid.UUID, uuid.UUID, error) {
	if idStr == "" {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if clientIDStr == "" {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid id format")
	}
	clientID, err := uuid.Parse(clientIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}
	return id, clientID, nil
}

func (s *Server) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrTemplateNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrTemplateNotApproved):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrInvalidTemplateName):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidTemplateBody):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrMissingVariables):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrRenderedTooLong):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrVariableValueTooLong):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidStatus):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrDuplicateTemplateName):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		s.logger.Error().Err(err).Msg("internal error")
		return status.Error(codes.Internal, err.Error())
	}
}

func templateToProto(tmpl *domain.Template) *templatev1.TemplateInfo {
	return &templatev1.TemplateInfo{
		Id:              tmpl.ID.String(),
		ClientId:        tmpl.ClientID.String(),
		Name:            tmpl.Name,
		Body:            tmpl.Body,
		Variables:       tmpl.Variables,
		Status:          tmpl.Status,
		RejectionReason: tmpl.RejectionReason,
		CreatedAt:       timestamppb.New(tmpl.CreatedAt),
		UpdatedAt:       timestamppb.New(tmpl.UpdatedAt),
	}
}

func auditToProto(entry *domain.AuditEntry) *templatev1.AuditEntry {
	proto := &templatev1.AuditEntry{
		Id:        entry.ID.String(),
		Action:    entry.Action,
		OldBody:   entry.OldBody,
		NewBody:   entry.NewBody,
		ActorType: entry.ActorType,
		Reason:    entry.Reason,
		CreatedAt: timestamppb.New(entry.CreatedAt),
	}
	if entry.TemplateID != nil {
		proto.TemplateId = entry.TemplateID.String()
	}
	if entry.ActorID != nil {
		proto.ActorId = entry.ActorID.String()
	}
	return proto
}
