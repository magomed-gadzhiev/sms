package grpc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

// TestMapCascadeErr проверяет, что domain-sentinel'ы корректно мапятся в gRPC коды
// (regression-guard для C.6b — раньше эти ошибки уходили как codes.Unknown → 500).
func TestMapCascadeErr(t *testing.T) {
	tests := []struct {
		name string
		in   error
		want codes.Code
	}{
		{"nil", nil, codes.OK},
		{"DeliveryNotFound", domain.ErrDeliveryNotFound, codes.NotFound},
		{"StrategyNotFound", domain.ErrStrategyNotFound, codes.NotFound},
		{"ChannelNotFound", domain.ErrChannelNotFound, codes.NotFound},
		{"AttemptNotFound", domain.ErrAttemptNotFound, codes.NotFound},
		{"DuplicateChannelType", domain.ErrDuplicateChannelType, codes.AlreadyExists},
		{"DuplicateStrategyName", domain.ErrDuplicateStrategyName, codes.AlreadyExists},
		{"AttemptAlreadyExists", domain.ErrAttemptAlreadyExists, codes.AlreadyExists},
		{"StrategyHasActiveDeliveries", domain.ErrStrategyHasActiveDeliveries, codes.FailedPrecondition},
		{"ChannelNotSupported", domain.ErrChannelNotSupported, codes.FailedPrecondition},
		{"InvalidTransition", domain.ErrInvalidTransition, codes.FailedPrecondition},
		{"DeliveryConflict", domain.ErrDeliveryConflict, codes.FailedPrecondition},
		{"LateDuplicate", domain.ErrLateDuplicate, codes.FailedPrecondition},
		{"unknown error → Internal", errors.New("random db error"), codes.Internal},
		// Wrapped через %w — errors.Is должен находить sentinel сквозь обёртку.
		{"wrapped NotFound", fmt.Errorf("wrap: %w", domain.ErrStrategyNotFound), codes.NotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapCascadeErr(tc.in)
			if tc.want == codes.OK {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			st, ok := status.FromError(got)
			if !ok {
				t.Fatalf("expected gRPC status error, got %T: %v", got, got)
			}
			if st.Code() != tc.want {
				t.Fatalf("expected %s, got %s (msg=%q)", tc.want, st.Code(), st.Message())
			}
		})
	}
}

// TestHandler_ValidationReturnsInvalidArgument проверяет, что edge-валидация
// в gRPC-handler'е возвращает codes.InvalidArgument (мапится gateway'ом в HTTP 400),
// а не codes.Unknown (→ 500). Regression-guard для C.6.
//
// Поскольку валидация UUID/enum срабатывает ДО вызова application-сервисов,
// тест работает с пустым Server{} — сервисы не дёргаются.
func TestHandler_ValidationReturnsInvalidArgument(t *testing.T) {
	const validUUID = "11111111-1111-1111-1111-111111111111"
	const badUUID = "not-a-uuid"

	tests := []struct {
		name string
		call func(s *Server) error
	}{
		{
			name: "CreateDelivery/bad client_id",
			call: func(s *Server) error {
				_, err := s.CreateDelivery(context.Background(), &cascadev1.CreateDeliveryRequest{
					ClientId: badUUID, StrategyId: validUUID,
				})
				return err
			},
		},
		{
			name: "CreateDelivery/bad strategy_id",
			call: func(s *Server) error {
				_, err := s.CreateDelivery(context.Background(), &cascadev1.CreateDeliveryRequest{
					ClientId: validUUID, StrategyId: badUUID,
				})
				return err
			},
		},
		{
			name: "ListDeliveries/bad client_id",
			call: func(s *Server) error {
				_, err := s.ListDeliveries(context.Background(), &cascadev1.ListDeliveriesRequest{
					ClientId: badUUID,
				})
				return err
			},
		},
		{
			name: "GetDeliveryStats/bad client_id",
			call: func(s *Server) error {
				_, err := s.GetDeliveryStats(context.Background(), &cascadev1.GetDeliveryStatsRequest{
					ClientId: badUUID,
				})
				return err
			},
		},
		{
			name: "GetChannel/bad channel_id",
			call: func(s *Server) error {
				_, err := s.GetChannel(context.Background(), &cascadev1.GetChannelRequest{
					ChannelId: badUUID,
				})
				return err
			},
		},
		{
			name: "CreateChannel/bad channel_type",
			call: func(s *Server) error {
				_, err := s.CreateChannel(context.Background(), &cascadev1.CreateChannelRequest{
					ChannelType: "wat",
				})
				return err
			},
		},
		{
			name: "UpdateChannel/bad channel_id",
			call: func(s *Server) error {
				_, err := s.UpdateChannel(context.Background(), &cascadev1.UpdateChannelRequest{
					ChannelId: badUUID,
				})
				return err
			},
		},
		{
			name: "ToggleChannel/bad channel_id",
			call: func(s *Server) error {
				_, err := s.ToggleChannel(context.Background(), &cascadev1.ToggleChannelRequest{
					ChannelId: badUUID,
				})
				return err
			},
		},
		{
			name: "GetStrategy/bad strategy_id",
			call: func(s *Server) error {
				_, err := s.GetStrategy(context.Background(), &cascadev1.GetStrategyRequest{
					StrategyId: badUUID,
				})
				return err
			},
		},
		{
			name: "CreateStrategy/bad mode",
			call: func(s *Server) error {
				_, err := s.CreateStrategy(context.Background(), &cascadev1.CreateStrategyRequest{
					Mode: "wat",
				})
				return err
			},
		},
		{
			name: "CreateStrategy/bad channel_id in step",
			call: func(s *Server) error {
				_, err := s.CreateStrategy(context.Background(), &cascadev1.CreateStrategyRequest{
					Mode: "sequential",
					Steps: []*cascadev1.CreateStrategyStepInput{
						{ChannelId: badUUID, StepOrder: 1},
					},
				})
				return err
			},
		},
		{
			name: "UpdateStrategy/bad strategy_id",
			call: func(s *Server) error {
				_, err := s.UpdateStrategy(context.Background(), &cascadev1.UpdateStrategyRequest{
					StrategyId: badUUID, Mode: "sequential",
				})
				return err
			},
		},
		{
			name: "UpdateStrategy/bad channel_id in step",
			call: func(s *Server) error {
				_, err := s.UpdateStrategy(context.Background(), &cascadev1.UpdateStrategyRequest{
					StrategyId: validUUID,
					Mode:       "sequential",
					Steps: []*cascadev1.CreateStrategyStepInput{
						{ChannelId: badUUID, StepOrder: 1},
					},
				})
				return err
			},
		},
		{
			name: "DeleteStrategy/bad strategy_id",
			call: func(s *Server) error {
				_, err := s.DeleteStrategy(context.Background(), &cascadev1.DeleteStrategyRequest{
					StrategyId: badUUID,
				})
				return err
			},
		},
		{
			name: "GetOperatorChannelSupport/bad operator_id",
			call: func(s *Server) error {
				_, err := s.GetOperatorChannelSupport(context.Background(), &cascadev1.GetOCSRequest{
					OperatorId: badUUID,
				})
				return err
			},
		},
		{
			name: "UpdateOperatorChannelSupport/bad operator_id",
			call: func(s *Server) error {
				_, err := s.UpdateOperatorChannelSupport(context.Background(), &cascadev1.UpdateOCSRequest{
					OperatorId: badUUID, ChannelType: "sms",
				})
				return err
			},
		},
		{
			name: "UpdateOperatorChannelSupport/bad channel_type",
			call: func(s *Server) error {
				_, err := s.UpdateOperatorChannelSupport(context.Background(), &cascadev1.UpdateOCSRequest{
					OperatorId: validUUID, ChannelType: "wat",
				})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			err := tc.call(s)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("expected gRPC status error, got %T: %v", err, err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Fatalf("expected codes.InvalidArgument, got %s: %s", st.Code(), st.Message())
			}
		})
	}
}
