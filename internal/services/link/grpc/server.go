package grpc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/internal/services/link/application"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

type LinkGrpcServer struct {
	linkv1.UnimplementedLinkServiceServer
	service *application.LinkService
}

func NewLinkGrpcServer(service *application.LinkService) *LinkGrpcServer {
	return &LinkGrpcServer{service: service}
}

func (s *LinkGrpcServer) ShortenURL(ctx context.Context, req *linkv1.ShortenRequest) (*linkv1.ShortenResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}

	var messageID, campaignID, recipientID *uuid.UUID
	if req.MessageId != "" {
		id, _ := uuid.Parse(req.MessageId)
		messageID = &id
	}
	if req.CampaignId != "" {
		id, _ := uuid.Parse(req.CampaignId)
		campaignID = &id
	}
	if req.RecipientId != "" {
		id, _ := uuid.Parse(req.RecipientId)
		recipientID = &id
	}

	shortURL, code, err := s.service.ShortenURL(ctx, clientID, req.OriginalUrl, messageID, campaignID, recipientID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "shorten failed: %s", err)
	}

	return &linkv1.ShortenResponse{ShortUrl: shortURL, Code: code}, nil
}

func (s *LinkGrpcServer) ShortenBatch(ctx context.Context, req *linkv1.ShortenBatchRequest) (*linkv1.ShortenBatchResponse, error) {
	resp := &linkv1.ShortenBatchResponse{}
	for _, r := range req.Urls {
		result, err := s.ShortenURL(ctx, r)
		if err != nil {
			return nil, err
		}
		resp.Results = append(resp.Results, result)
	}
	return resp, nil
}

func (s *LinkGrpcServer) GetLinkStats(ctx context.Context, req *linkv1.LinkStatsRequest) (*linkv1.LinkStatsResponse, error) {
	// Placeholder — will be populated via click_repo
	return &linkv1.LinkStatsResponse{}, nil
}

// DomainGrpcServer implements the DomainService gRPC server.
type DomainGrpcServer struct {
	linkv1.UnimplementedDomainServiceServer
	service *application.LinkService
}

func NewDomainGrpcServer(service *application.LinkService) *DomainGrpcServer {
	return &DomainGrpcServer{service: service}
}

func (s *DomainGrpcServer) AddDomain(ctx context.Context, req *linkv1.AddDomainRequest) (*linkv1.Domain, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	d, err := s.service.AddDomain(ctx, clientID, req.Domain)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add domain: %s", err)
	}
	return domainToProto(d), nil
}

func (s *DomainGrpcServer) ListDomains(ctx context.Context, req *linkv1.ListDomainsRequest) (*linkv1.ListDomainsResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	domains, err := s.service.ListDomains(ctx, clientID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list domains: %s", err)
	}
	resp := &linkv1.ListDomainsResponse{}
	for _, d := range domains {
		resp.Domains = append(resp.Domains, domainToProto(d))
	}
	return resp, nil
}

func (s *DomainGrpcServer) DeleteDomain(ctx context.Context, req *linkv1.DeleteDomainRequest) (*linkv1.Empty, error) {
	id, err := uuid.Parse(req.DomainId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid domain_id")
	}
	if err := s.service.DeleteDomain(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete domain: %s", err)
	}
	return &linkv1.Empty{}, nil
}

func domainToProto(d *domain.ClientDomain) *linkv1.Domain {
	proto := &linkv1.Domain{
		Id:           d.ID.String(),
		ClientId:     d.ClientID.String(),
		Domain:       d.Domain,
		Status:       d.Status,
		DnsTxtRecord: d.DNSTxtRecord,
		CreatedAt:    timestamppb.New(d.CreatedAt),
	}
	if d.DNSVerifiedAt != nil {
		proto.DnsVerifiedAt = timestamppb.New(*d.DNSVerifiedAt)
	}
	if d.SSLExpiresAt != nil {
		proto.SslExpiresAt = timestamppb.New(*d.SSLExpiresAt)
	}
	return proto
}
