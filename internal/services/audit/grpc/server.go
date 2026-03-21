package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/auditv1"
	"github.com/smpp-server/smpp-server/internal/services/audit/domain"
)

// Server implements the auditv1.AuditServiceServer interface.
type Server struct {
	auditv1.UnimplementedAuditServiceServer
	repo domain.AuditLogRepository
}

// NewServer creates a new audit gRPC server.
func NewServer(repo domain.AuditLogRepository) *Server {
	return &Server{repo: repo}
}

// QueryAuditLog handles the QueryAuditLog RPC.
func (s *Server) QueryAuditLog(ctx context.Context, req *auditv1.QueryAuditLogRequest) (*auditv1.QueryAuditLogResponse, error) {
	if req.TenantId == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	filters := &domain.AuditLogFilters{
		TenantID: req.TenantId,
		Action:   req.Action,
		UserID:   req.UserId,
		Page:     req.Page,
		PerPage:  req.PerPage,
	}

	if req.DateFrom != nil {
		t := req.DateFrom.AsTime()
		filters.DateFrom = &t
	}
	if req.DateTo != nil {
		t := req.DateTo.AsTime()
		filters.DateTo = &t
	}

	entries, total, err := s.repo.QueryAuditLog(ctx, filters)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query audit log: %v", err)
	}

	perPage := filters.PerPage
	if perPage < 1 {
		perPage = 20
	}
	totalPages := int32(total) / perPage
	if int32(total)%perPage > 0 {
		totalPages++
	}

	pbEntries := make([]*auditv1.AuditLogEntry, 0, len(entries))
	for _, e := range entries {
		pbEntries = append(pbEntries, &auditv1.AuditLogEntry{
			Id:           e.ID,
			TenantId:     e.TenantID,
			UserId:       e.UserID,
			Action:       e.Action,
			ResourceType: e.ResourceType,
			ResourceId:   e.ResourceID,
			Details:      e.Details,
			IpAddress:    e.IPAddress,
			CreatedAt:    timestamppb.New(e.CreatedAt),
		})
	}

	return &auditv1.QueryAuditLogResponse{
		Entries:    pbEntries,
		Total:      int32(total),
		Page:       filters.Page,
		TotalPages: totalPages,
	}, nil
}
