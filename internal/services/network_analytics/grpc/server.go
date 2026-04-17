package grpc

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	networkanalyticsv1 "github.com/smpp-server/smpp-server/api/proto/networkanalyticsv1"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/application"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// Server implements the NetworkAnalyticsServiceServer gRPC interface.
type Server struct {
	networkanalyticsv1.UnimplementedNetworkAnalyticsServiceServer
	service *application.NetworkAnalyticsService
}

// NewServer creates a new gRPC server for NetworkAnalyticsService.
func NewServer(service *application.NetworkAnalyticsService) *Server {
	return &Server{service: service}
}

// GetStatistics returns aggregated statistics rows with KPIs and pagination.
func (s *Server) GetStatistics(ctx context.Context, req *networkanalyticsv1.StatisticsRequest) (*networkanalyticsv1.StatisticsResponse, error) {
	filter := protoFilterToDomain(req.Filter)
	filter.PartnerID = req.PartnerId
	filter.Normalize()

	result, err := s.service.GetStatistics(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get statistics: %v", err)
	}

	return &networkanalyticsv1.StatisticsResponse{
		Kpis:       domainKPIsToProto(result.KPIs),
		Rows:       domainStatRowsToProto(result.Rows),
		Pagination: domainPaginationToProto(result.Pagination),
	}, nil
}

// GetAnalyticsSummary returns analytics KPIs, trends, signals and rows.
func (s *Server) GetAnalyticsSummary(ctx context.Context, req *networkanalyticsv1.AnalyticsRequest) (*networkanalyticsv1.AnalyticsResponse, error) {
	filter := protoFilterToDomain(req.Filter)
	filter.PartnerID = req.PartnerId
	filter.Normalize()

	result, err := s.service.GetAnalyticsSummary(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get analytics summary: %v", err)
	}

	return &networkanalyticsv1.AnalyticsResponse{
		Kpis:         domainKPIsToProto(result.KPIs),
		PreviousKpis: domainKPIsToProto(result.PreviousKPIs),
		Trends:       domainTrendsToProto(result.Trends),
		Signals:      domainSignalsToProto(result.Signals),
		Rows:         domainStatRowsToProto(result.Rows),
		Pagination:   domainPaginationToProto(result.Pagination),
	}, nil
}

// GetMonitoringMetrics returns real-time monitoring data for network providers.
func (s *Server) GetMonitoringMetrics(ctx context.Context, req *networkanalyticsv1.MonitoringRequest) (*networkanalyticsv1.MonitoringResponse, error) {
	filter := protoFilterToDomain(req.Filter)
	filter.PartnerID = req.PartnerId
	filter.Normalize()

	result, err := s.service.GetMonitoringMetrics(ctx, filter, req.HideHealthy)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get monitoring metrics: %v", err)
	}

	chart := make([]*networkanalyticsv1.MetricPoint, len(result.Chart))
	for i, p := range result.Chart {
		chart[i] = &networkanalyticsv1.MetricPoint{
			Timestamp: p.Timestamp,
			Value:     p.Value,
		}
	}

	return &networkanalyticsv1.MonitoringResponse{
		Kpis:       domainKPIsToProto(result.KPIs),
		Rows:       domainMonitorRowsToProto(result.Rows),
		Chart:      chart,
		Pagination: domainPaginationToProto(result.Pagination),
	}, nil
}

// GetDrillDown returns detailed breakdown for a specific slice value.
func (s *Server) GetDrillDown(ctx context.Context, req *networkanalyticsv1.DrillDownRequest) (*networkanalyticsv1.DrillDownResponse, error) {
	if req.SliceType == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slice_type is required")
	}

	filter := protoFilterToDomain(req.Filter)
	filter.PartnerID = req.PartnerId
	filter.Normalize()

	params := &domain.DrillDownParams{
		PartnerID:   req.PartnerId,
		Filter:      filter,
		SliceType:   req.SliceType,
		SliceValue:  req.SliceValue,
		DetailView:  req.DetailView,
		ParentType:  req.ParentType,
		ParentValue: req.ParentValue,
	}

	result, err := s.service.GetDrillDown(ctx, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get drill down: %v", err)
	}

	return &networkanalyticsv1.DrillDownResponse{
		Summary: domainKPIsToProto(result.Summary),
		Rows:    domainStatRowsToProto(result.Rows),
		Trends:  domainTrendsToProto(result.Trends),
		Health:  result.Health,
	}, nil
}

// StartExport initiates an async export job and returns the job ID.
func (s *Server) StartExport(ctx context.Context, req *networkanalyticsv1.ExportRequest) (*networkanalyticsv1.ExportResponse, error) {
	if req.Format == "" {
		return nil, status.Errorf(codes.InvalidArgument, "format is required")
	}

	filter := protoFilterToDomain(req.Filter)
	filter.PartnerID = req.PartnerId
	filter.Normalize()

	filtersJSON := "{}"
	if req.Filter != nil {
		if b, err := json.Marshal(req.Filter); err == nil {
			filtersJSON = string(b)
		}
	}

	job := &domain.ExportJob{
		PartnerID: req.PartnerId,
		UserID:    req.UserId,
		Mode:      req.Mode,
		Filters:   filtersJSON,
		Format:    req.Format,
		Status:    "pending",
	}

	jobID, err := s.service.StartExport(ctx, job)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "start export: %v", err)
	}

	return &networkanalyticsv1.ExportResponse{
		JobId: jobID,
	}, nil
}

// GetExportStatus returns the current status of an export job.
func (s *Server) GetExportStatus(ctx context.Context, req *networkanalyticsv1.ExportStatusRequest) (*networkanalyticsv1.ExportStatusResponse, error) {
	if req.JobId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "job_id is required")
	}

	job, err := s.service.GetExportStatus(ctx, req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get export status: %v", err)
	}

	resp := &networkanalyticsv1.ExportStatusResponse{
		Status:   job.Status,
		RowCount: int32(job.RowCount),
		Error:    job.Error,
	}
	if job.Status == "done" {
		resp.DownloadUrl = job.FilePath
	}

	return resp, nil
}

// ListSavedViews returns all saved views for a partner/user combination.
func (s *Server) ListSavedViews(ctx context.Context, req *networkanalyticsv1.ListViewsRequest) (*networkanalyticsv1.ListViewsResponse, error) {
	views, err := s.service.ListViews(ctx, req.PartnerId, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list saved views: %v", err)
	}

	protoViews := make([]*networkanalyticsv1.SavedView, len(views))
	for i, v := range views {
		protoViews[i] = domainViewToProto(v)
	}

	return &networkanalyticsv1.ListViewsResponse{
		Views: protoViews,
	}, nil
}

// SaveView creates or updates a saved view configuration.
func (s *Server) SaveView(ctx context.Context, req *networkanalyticsv1.SaveViewRequest) (*networkanalyticsv1.SaveViewResponse, error) {
	if req.View == nil {
		return nil, status.Errorf(codes.InvalidArgument, "view is required")
	}
	if req.View.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "view name is required")
	}

	domainView := protoViewToDomain(req.View, req.PartnerId, req.UserId)

	saved, err := s.service.SaveView(ctx, domainView)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save view: %v", err)
	}

	return &networkanalyticsv1.SaveViewResponse{
		View: domainViewToProto(*saved),
	}, nil
}

// DeleteView removes a saved view by ID.
func (s *Server) DeleteView(ctx context.Context, req *networkanalyticsv1.DeleteViewRequest) (*networkanalyticsv1.DeleteViewResponse, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}
	if err := s.service.DeleteView(ctx, req.Id, req.PartnerId, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete view: %v", err)
	}

	return &networkanalyticsv1.DeleteViewResponse{}, nil
}

// --- Helper functions: proto → domain ---

// protoFilterToDomain converts a proto SharedFilter to a domain SharedFilter.
// Returns an empty filter (never nil) if pb is nil.
func protoFilterToDomain(pb *networkanalyticsv1.SharedFilter) *domain.SharedFilter {
	f := &domain.SharedFilter{}
	if pb == nil {
		return f
	}

	f.PeriodPreset = pb.PeriodPreset
	f.GroupBy = pb.GroupBy
	f.Login = pb.Login
	f.ServiceType = pb.ServiceType
	f.Operator = pb.Operator
	f.Channel = pb.Channel
	f.SenderName = pb.SenderName
	f.SenderPaid = pb.SenderPaid
	f.International = pb.International
	f.TrafficType = pb.TrafficType
	f.Status = pb.Status
	f.PriceRange = pb.PriceRange
	f.Method = pb.Method
	f.Provider = pb.Provider
	f.Country = pb.Country
	f.Manager = pb.Manager
	f.ErrorCode = pb.ErrorCode
	f.Page = int(pb.Page)
	f.PageSize = int(pb.PageSize)
	f.SortBy = pb.SortBy
	f.SortDir = pb.SortDir

	if pb.DateFrom != 0 {
		f.DateFrom = time.Unix(pb.DateFrom, 0).UTC()
	}
	if pb.DateTo != 0 {
		f.DateTo = time.Unix(pb.DateTo, 0).UTC()
	}

	return f
}

// protoViewToDomain converts a proto SavedView to a domain SavedView.
func protoViewToDomain(pb *networkanalyticsv1.SavedView, partnerID, userID int64) *domain.SavedView {
	v := &domain.SavedView{
		ID:        pb.Id,
		PartnerID: partnerID,
		Name:      pb.Name,
		Mode:      pb.Mode,
		Filters:   pb.FiltersJson,
		GroupBy:   pb.GroupBy,
		SortBy:    pb.SortBy,
		SortDir:   pb.SortDir,
		Columns:   pb.Columns,
		IsDefault: pb.IsDefault,
	}
	if userID != 0 {
		v.UserID = &userID
	}
	return v
}

// --- Helper functions: domain → proto ---

func domainKPIsToProto(kpis []domain.KPI) []*networkanalyticsv1.KPI {
	out := make([]*networkanalyticsv1.KPI, len(kpis))
	for i, k := range kpis {
		out[i] = &networkanalyticsv1.KPI{
			Name:   k.Name,
			Value:  k.Value,
			Delta:  k.Delta,
			Status: k.Status,
		}
	}
	return out
}

func domainStatRowsToProto(rows []domain.StatRow) []*networkanalyticsv1.StatRow {
	out := make([]*networkanalyticsv1.StatRow, len(rows))
	for i, r := range rows {
		out[i] = &networkanalyticsv1.StatRow{
			Slice:     r.Slice,
			Total:     r.Total,
			Sent:      r.Sent,
			Delivered: r.Delivered,
			Failed:    r.Failed,
			Pending:   r.Pending,
			Timeout:   r.Timeout,
			Error:     r.Error,
			DlrRate:   r.DLRRate,
			Revenue:   r.Revenue,
			Cost:      r.Cost,
			Profit:    r.Profit,
			Margin:    r.Margin,
			Health:    r.Health,
			Alerts:    r.Alerts,
		}
	}
	return out
}

func domainMonitorRowsToProto(rows []domain.MonitorRow) []*networkanalyticsv1.MonitorRow {
	out := make([]*networkanalyticsv1.MonitorRow, len(rows))
	for i, r := range rows {
		out[i] = &networkanalyticsv1.MonitorRow{
			Slice:         r.Slice,
			Throughput:    r.Throughput,
			Sent:          r.Sent,
			Delivered:     r.Delivered,
			Pending:       r.Pending,
			Timeout:       r.Timeout,
			Error:         r.Error,
			DlrLatencyP50: r.DLRLatencyP50,
			DlrLatencyP95: r.DLRLatencyP95,
			DlrRate:       r.DLRRate,
			TopError:      r.TopError,
			Health:        r.Health,
		}
	}
	return out
}

func domainTrendsToProto(trends []domain.Trend) []*networkanalyticsv1.Trend {
	out := make([]*networkanalyticsv1.Trend, len(trends))
	for i, t := range trends {
		points := make([]*networkanalyticsv1.MetricPoint, len(t.Points))
		for j, p := range t.Points {
			points[j] = &networkanalyticsv1.MetricPoint{
				Timestamp: p.Timestamp,
				Value:     p.Value,
			}
		}
		out[i] = &networkanalyticsv1.Trend{
			Metric: t.Metric,
			Points: points,
		}
	}
	return out
}

func domainSignalsToProto(signals []domain.Signal) []*networkanalyticsv1.Signal {
	out := make([]*networkanalyticsv1.Signal, len(signals))
	for i, s := range signals {
		out[i] = &networkanalyticsv1.Signal{
			Severity:  s.Severity,
			Text:      s.Text,
			LinkType:  s.LinkType,
			LinkValue: s.LinkValue,
		}
	}
	return out
}

func domainPaginationToProto(p domain.Pagination) *networkanalyticsv1.Pagination {
	return &networkanalyticsv1.Pagination{
		Page:       int32(p.Page),
		PageSize:   int32(p.PageSize),
		TotalRows:  int32(p.TotalRows),
		TotalPages: int32(p.TotalPages),
	}
}

func domainViewToProto(v domain.SavedView) *networkanalyticsv1.SavedView {
	pb := &networkanalyticsv1.SavedView{
		Id:          v.ID,
		Name:        v.Name,
		Mode:        v.Mode,
		FiltersJson: v.Filters,
		GroupBy:     v.GroupBy,
		SortBy:      v.SortBy,
		SortDir:     v.SortDir,
		Columns:     v.Columns,
		IsDefault:   v.IsDefault,
		IsTemplate:  v.UserID == nil,
	}
	return pb
}
