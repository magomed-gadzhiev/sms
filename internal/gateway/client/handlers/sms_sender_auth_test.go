package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
)

// mockSenderNameClient implements sendernamev1.SenderNameServiceClient for
// testing the sender-name authorization path (Bug #7).
type mockSenderNameClient struct {
	mock.Mock
	sendernamev1.SenderNameServiceClient // embed for forward-compat methods we don't use
}

func (m *mockSenderNameClient) ListSenderNames(ctx context.Context, in *sendernamev1.ListSenderNamesRequest, _ ...grpc.CallOption) (*sendernamev1.ListSenderNamesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sendernamev1.ListSenderNamesResponse), args.Error(1)
}

func newHandlerWithSenderNameClient(sn sendernamev1.SenderNameServiceClient) *SMSHandlers {
	return NewSMSHandlers(nil, nil, nil, sn)
}

func TestResolveSenderName_Approved(t *testing.T) {
	snID := uuid.New().String()
	clientID := uuid.New().String()

	sn := &mockSenderNameClient{}
	// First call: status=approved filter → returns the approved row.
	sn.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
		return in.ClientId == clientID && in.Status == "approved"
	})).Return(&sendernamev1.ListSenderNamesResponse{
		SenderNames: []*sendernamev1.SenderNameInfo{
			{Id: snID, Name: "MYBRAND", Status: "approved"},
		},
	}, nil).Once()

	h := newHandlerWithSenderNameClient(sn)
	id, appErr := h.resolveSenderName(context.Background(), clientID, "MYBRAND")
	require.Nil(t, appErr)
	assert.Equal(t, snID, id)
	sn.AssertExpectations(t)
}

func TestResolveSenderName_Unknown(t *testing.T) {
	clientID := uuid.New().String()

	sn := &mockSenderNameClient{}
	// First call (approved filter): empty list.
	sn.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
		return in.Status == "approved"
	})).Return(&sendernamev1.ListSenderNamesResponse{}, nil).Once()
	// Second call (no filter): also empty — truly unknown.
	sn.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
		return in.Status == ""
	})).Return(&sendernamev1.ListSenderNamesResponse{}, nil).Once()

	h := newHandlerWithSenderNameClient(sn)
	id, appErr := h.resolveSenderName(context.Background(), clientID, "GHOST")
	require.NotNil(t, appErr)
	assert.Empty(t, id)
	assert.Contains(t, appErr.Message, "не зарегистрирован")
	sn.AssertExpectations(t)
}

func TestResolveSenderName_Pending(t *testing.T) {
	clientID := uuid.New().String()

	sn := &mockSenderNameClient{}
	sn.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
		return in.Status == "approved"
	})).Return(&sendernamev1.ListSenderNamesResponse{}, nil).Once()
	sn.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
		return in.Status == ""
	})).Return(&sendernamev1.ListSenderNamesResponse{
		SenderNames: []*sendernamev1.SenderNameInfo{
			{Id: uuid.New().String(), Name: "PENDO", Status: "pending"},
		},
	}, nil).Once()

	h := newHandlerWithSenderNameClient(sn)
	id, appErr := h.resolveSenderName(context.Background(), clientID, "PENDO")
	require.NotNil(t, appErr)
	assert.Empty(t, id)
	assert.Contains(t, appErr.Message, "не одобрен")
	assert.Contains(t, appErr.Message, "pending")
	sn.AssertExpectations(t)
}

func TestResolveSenderName_TransportError(t *testing.T) {
	clientID := uuid.New().String()

	sn := &mockSenderNameClient{}
	sn.On("ListSenderNames", mock.Anything, mock.Anything).
		Return(nil, errors.New("boom")).Once()

	h := newHandlerWithSenderNameClient(sn)
	id, appErr := h.resolveSenderName(context.Background(), clientID, "ANY")
	require.NotNil(t, appErr)
	assert.Empty(t, id)
	assert.Contains(t, appErr.Message, "проверки sender name")
	sn.AssertExpectations(t)
}

func TestResolveSenderName_NilClient(t *testing.T) {
	// When senderNameClient is nil (legacy tests), skip the check and return
	// empty string + nil error.
	h := NewSMSHandlers(nil, nil, nil, nil)
	id, appErr := h.resolveSenderName(context.Background(), uuid.New().String(), "ANY")
	assert.Empty(t, id)
	assert.Nil(t, appErr)
}
