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

	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	"github.com/smpp-server/smpp-server/internal/services/template/application"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type SenderNameHandler struct {
	sendernamev1.UnimplementedSenderNameServiceServer
	svc    *application.SenderNameService
	logger zerolog.Logger
}

func NewSenderNameHandler(svc *application.SenderNameService) *SenderNameHandler {
	return &SenderNameHandler{
		svc:    svc,
		logger: log.With().Str("component", "sender-name-grpc-handler").Logger(),
	}
}

func (h *SenderNameHandler) CreateSenderName(ctx context.Context, req *sendernamev1.CreateSenderNameRequest) (*sendernamev1.CreateSenderNameResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id format")
	}

	sn, err := h.svc.RegisterSenderName(ctx, clientID, companyID, req.Name)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.CreateSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}

func (h *SenderNameHandler) UpdateSenderName(ctx context.Context, req *sendernamev1.UpdateSenderNameRequest) (*sendernamev1.UpdateSenderNameResponse, error) {
	id, clientID, err := parseTwoUUIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	sn, err := h.svc.UpdateSenderName(ctx, id, clientID, req.Name)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.UpdateSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}

func (h *SenderNameHandler) GetSenderName(ctx context.Context, req *sendernamev1.GetSenderNameRequest) (*sendernamev1.GetSenderNameResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	// Пустой ClientId — режим администратора (без проверки владельца)
	if req.ClientId == "" {
		sn, err := h.svc.GetSenderNameAdmin(ctx, id)
		if err != nil {
			return nil, h.mapError(err)
		}
		return &sendernamev1.GetSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}
	sn, err := h.svc.GetSenderName(ctx, id, clientID)
	if err != nil {
		return nil, h.mapError(err)
	}
	return &sendernamev1.GetSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}

func (h *SenderNameHandler) ListSenderNames(ctx context.Context, req *sendernamev1.ListSenderNamesRequest) (*sendernamev1.ListSenderNamesResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	senderNames, total, err := h.svc.ListSenderNames(ctx, clientID, req.Status, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, h.mapError(err)
	}

	protoNames := make([]*sendernamev1.SenderNameInfo, len(senderNames))
	for i, sn := range senderNames {
		protoNames[i] = senderNameToProto(sn)
	}

	return &sendernamev1.ListSenderNamesResponse{
		SenderNames: protoNames,
		Total:       int32(total),
		Limit:       req.Limit,
		Offset:      req.Offset,
	}, nil
}

func (h *SenderNameHandler) ResubmitSenderName(ctx context.Context, req *sendernamev1.ResubmitSenderNameRequest) (*sendernamev1.ResubmitSenderNameResponse, error) {
	id, clientID, err := parseTwoUUIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	sn, err := h.svc.ResubmitSenderName(ctx, id, clientID)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.ResubmitSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}

func (h *SenderNameHandler) GetSenderNameHistory(ctx context.Context, req *sendernamev1.GetSenderNameHistoryRequest) (*sendernamev1.GetSenderNameHistoryResponse, error) {
	if req.SenderNameId == "" {
		return nil, status.Error(codes.InvalidArgument, "sender_name_id is required")
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	senderNameID, err := uuid.Parse(req.SenderNameId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sender_name_id format")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	entries, total, err := h.svc.GetSenderNameHistory(ctx, senderNameID, clientID, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, h.mapError(err)
	}

	protoEntries := make([]*sendernamev1.SenderNameHistoryEntry, len(entries))
	for i, e := range entries {
		protoEntries[i] = historyEntryToProto(e)
	}

	return &sendernamev1.GetSenderNameHistoryResponse{
		Entries: protoEntries,
		Total:   int32(total),
	}, nil
}

func (h *SenderNameHandler) ApproveSenderName(ctx context.Context, req *sendernamev1.ApproveSenderNameRequest) (*sendernamev1.ApproveSenderNameResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.ActorId == "" {
		return nil, status.Error(codes.InvalidArgument, "actor_id is required")
	}
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}
	actorID, err := uuid.Parse(req.ActorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid actor_id format")
	}

	sn, err := h.svc.ApproveSenderName(ctx, id, actorID)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.ApproveSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}

func (h *SenderNameHandler) RejectSenderName(ctx context.Context, req *sendernamev1.RejectSenderNameRequest) (*sendernamev1.RejectSenderNameResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.ActorId == "" {
		return nil, status.Error(codes.InvalidArgument, "actor_id is required")
	}
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}
	actorID, err := uuid.Parse(req.ActorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid actor_id format")
	}

	sn, err := h.svc.RejectSenderName(ctx, id, actorID, req.Reason)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.RejectSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}

func (h *SenderNameHandler) DeactivateSenderName(ctx context.Context, req *sendernamev1.DeactivateSenderNameRequest) (*sendernamev1.DeactivateSenderNameResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.ActorId == "" {
		return nil, status.Error(codes.InvalidArgument, "actor_id is required")
	}
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}
	actorID, err := uuid.Parse(req.ActorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid actor_id format")
	}

	sn, err := h.svc.DeactivateSenderName(ctx, id, actorID, req.Reason)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.DeactivateSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}

func (h *SenderNameHandler) ListAllSenderNames(ctx context.Context, req *sendernamev1.ListAllSenderNamesRequest) (*sendernamev1.ListAllSenderNamesResponse, error) {
	var clientID *uuid.UUID
	if req.ClientId != "" {
		parsed, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &parsed
	}

	senderNames, total, err := h.svc.ListAllSenderNames(ctx, clientID, req.Status, req.NameQuery, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, h.mapError(err)
	}

	protoNames := make([]*sendernamev1.SenderNameInfo, len(senderNames))
	for i, sn := range senderNames {
		protoNames[i] = senderNameToProto(sn)
	}

	return &sendernamev1.ListAllSenderNamesResponse{
		SenderNames: protoNames,
		Total:       int32(total),
		Limit:       req.Limit,
		Offset:      req.Offset,
	}, nil
}

func (h *SenderNameHandler) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrSenderNameNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDuplicateSenderName):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidSenderNameFormat):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidSenderNameStatus):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrSenderNameNotApproved):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		h.logger.Error().Err(err).Msg("internal error")
		return status.Error(codes.Internal, err.Error())
	}
}

func senderNameToProto(sn *domain.SenderName) *sendernamev1.SenderNameInfo {
	info := &sendernamev1.SenderNameInfo{
		Id:              sn.ID.String(),
		ClientId:        sn.ClientID.String(),
		CompanyId:       sn.CompanyID.String(),
		Name:            sn.Name,
		Status:          sn.Status,
		RejectionReason: sn.RejectionReason,
		CreatedAt:       timestamppb.New(sn.CreatedAt),
		UpdatedAt:       timestamppb.New(sn.UpdatedAt),
	}
	if sn.ReviewerID != nil {
		info.ReviewerId = sn.ReviewerID.String()
	}
	if sn.ReviewedAt != nil {
		info.ReviewedAt = timestamppb.New(*sn.ReviewedAt)
	}
	return info
}

func historyEntryToProto(e *domain.SenderNameStatusHistory) *sendernamev1.SenderNameHistoryEntry {
	entry := &sendernamev1.SenderNameHistoryEntry{
		Id:           e.ID.String(),
		SenderNameId: e.SenderNameID.String(),
		NewStatus:    e.NewStatus,
		ActorType:    e.ActorType,
		Comment:      e.Comment,
		CreatedAt:    timestamppb.New(e.CreatedAt),
	}
	if e.OldStatus != nil {
		entry.OldStatus = *e.OldStatus
	}
	if e.ActorID != nil {
		entry.ActorId = e.ActorID.String()
	}
	return entry
}

func parseTwoUUIDs(idStr, clientIDStr string) (uuid.UUID, uuid.UUID, error) {
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
