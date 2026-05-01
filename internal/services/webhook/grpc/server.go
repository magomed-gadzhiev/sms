// internal/services/webhook/grpc/server.go
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

	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/services/webhook/application"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

type Server struct {
	webhookv1.UnimplementedWebhookServiceServer
	webhookService *application.WebhookService
	logger         zerolog.Logger
}

func NewServer(webhookService *application.WebhookService) *Server {
	return &Server{
		webhookService: webhookService,
		logger:         log.With().Str("component", "webhook-grpc-server").Logger(),
	}
}

func (s *Server) CreateSubscription(ctx context.Context, req *webhookv1.CreateSubscriptionRequest) (*webhookv1.CreateSubscriptionResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Url == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}
	if len(req.EventTypes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "event_types is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	sub, err := s.webhookService.CreateSubscription(ctx, clientID, req.Url, req.EventTypes)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.CreateSubscriptionResponse{
		Subscription: subscriptionToProto(sub),
		Secret:       sub.Secret,
	}, nil
}

func (s *Server) GetSubscription(ctx context.Context, req *webhookv1.GetSubscriptionRequest) (*webhookv1.GetSubscriptionResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	sub, err := s.webhookService.GetSubscription(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.GetSubscriptionResponse{
		Subscription: subscriptionToProto(sub),
	}, nil
}

func (s *Server) ListSubscriptions(ctx context.Context, req *webhookv1.ListSubscriptionsRequest) (*webhookv1.ListSubscriptionsResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	subs, err := s.webhookService.ListSubscriptions(ctx, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	protoSubs := make([]*webhookv1.SubscriptionInfo, len(subs))
	for i, sub := range subs {
		protoSubs[i] = subscriptionToProto(sub)
	}

	return &webhookv1.ListSubscriptionsResponse{
		Subscriptions: protoSubs,
	}, nil
}

func (s *Server) UpdateSubscription(ctx context.Context, req *webhookv1.UpdateSubscriptionRequest) (*webhookv1.UpdateSubscriptionResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	var urlPtr *string
	if req.Url != "" {
		urlPtr = &req.Url
	}
	var activePtr *bool
	if req.Active != nil {
		activePtr = req.Active
	}

	var eventTypes []string
	if len(req.EventTypes) > 0 {
		eventTypes = req.EventTypes
	}

	sub, err := s.webhookService.UpdateSubscription(ctx, id, clientID, urlPtr, eventTypes, activePtr)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.UpdateSubscriptionResponse{
		Subscription: subscriptionToProto(sub),
	}, nil
}

func (s *Server) DeleteSubscription(ctx context.Context, req *webhookv1.DeleteSubscriptionRequest) (*webhookv1.DeleteSubscriptionResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	if err := s.webhookService.DeleteSubscription(ctx, id, clientID); err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.DeleteSubscriptionResponse{Success: true}, nil
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
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrMaxSubscriptionsReached):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, domain.ErrInvalidURL):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrPrivateURL):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrURLTooLong):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidEventType):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		s.logger.Error().Err(err).Msg("internal error")
		return status.Error(codes.Internal, err.Error())
	}
}

func subscriptionToProto(sub *domain.Subscription) *webhookv1.SubscriptionInfo {
	return &webhookv1.SubscriptionInfo{
		Id:         sub.ID.String(),
		ClientId:   sub.ClientID.String(),
		Url:        sub.URL,
		EventTypes: sub.EventTypes,
		Active:     sub.Active,
		CreatedAt:  timestamppb.New(sub.CreatedAt),
		UpdatedAt:  timestamppb.New(sub.UpdatedAt),
	}
}
