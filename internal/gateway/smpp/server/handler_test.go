package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	smppsession "github.com/smpp-server/smpp-server/internal/gateway/smpp/session"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// --- Mock AuthServiceClient ---

type mockAuthClient struct {
	mock.Mock
}

func (m *mockAuthClient) Authenticate(ctx context.Context, in *authv1.AuthenticateRequest, opts ...grpc.CallOption) (*authv1.AuthenticateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.AuthenticateResponse), args.Error(1)
}

func (m *mockAuthClient) ValidateToken(ctx context.Context, in *authv1.ValidateTokenRequest, opts ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ValidateTokenResponse), args.Error(1)
}

func (m *mockAuthClient) RefreshToken(ctx context.Context, in *authv1.RefreshTokenRequest, opts ...grpc.CallOption) (*authv1.RefreshTokenResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetPermissions(ctx context.Context, in *authv1.GetPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetPermissionsResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) CreateAPIKey(ctx context.Context, in *authv1.CreateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.CreateAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) RevokeAPIKey(ctx context.Context, in *authv1.RevokeAPIKeyRequest, opts ...grpc.CallOption) (*authv1.RevokeAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ListAPIKeys(ctx context.Context, in *authv1.ListAPIKeysRequest, opts ...grpc.CallOption) (*authv1.ListAPIKeysResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) UpdateAPIKey(ctx context.Context, in *authv1.UpdateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.UpdateAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ChangePassword(ctx context.Context, in *authv1.ChangePasswordRequest, opts ...grpc.CallOption) (*authv1.ChangePasswordResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) SetupTOTP(ctx context.Context, in *authv1.SetupTOTPRequest, opts ...grpc.CallOption) (*authv1.SetupTOTPResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) VerifyTOTP(ctx context.Context, in *authv1.VerifyTOTPRequest, opts ...grpc.CallOption) (*authv1.VerifyTOTPResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) DisableTOTP(ctx context.Context, in *authv1.DisableTOTPRequest, opts ...grpc.CallOption) (*authv1.DisableTOTPResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) RequestPasswordReset(ctx context.Context, in *authv1.RequestPasswordResetRequest, opts ...grpc.CallOption) (*authv1.RequestPasswordResetResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ResetPassword(ctx context.Context, in *authv1.ResetPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetPasswordResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) LoginWithSession(ctx context.Context, in *authv1.LoginWithSessionRequest, opts ...grpc.CallOption) (*authv1.LoginWithSessionResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) RegisterClient(ctx context.Context, in *authv1.RegisterClientRequest, opts ...grpc.CallOption) (*authv1.RegisterClientResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) UpdateUser(ctx context.Context, in *authv1.UpdateUserRequest, opts ...grpc.CallOption) (*authv1.UpdateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ResetUser2FA(ctx context.Context, in *authv1.ResetUser2FARequest, opts ...grpc.CallOption) (*authv1.ResetUser2FAResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ResetUserPassword(ctx context.Context, in *authv1.ResetUserPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetUserPasswordResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) CreateRole(ctx context.Context, in *authv1.CreateRoleRequest, opts ...grpc.CallOption) (*authv1.CreateRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) UpdateRole(ctx context.Context, in *authv1.UpdateRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) DeleteRole(ctx context.Context, in *authv1.DeleteRoleRequest, opts ...grpc.CallOption) (*authv1.DeleteRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ListRoles(ctx context.Context, in *authv1.ListRolesRequest, opts ...grpc.CallOption) (*authv1.ListRolesResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetRole(ctx context.Context, in *authv1.GetRoleRequest, opts ...grpc.CallOption) (*authv1.GetRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ListAllPermissions(ctx context.Context, in *authv1.ListAllPermissionsRequest, opts ...grpc.CallOption) (*authv1.ListAllPermissionsResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetUserPermissions(ctx context.Context, in *authv1.GetUserPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetUserPermissionsResponse, error) {
	return nil, nil
}

// --- Mock Producer ---

type mockProducer struct {
	mu            sync.Mutex
	publishCalled bool
	publishErr    error
}

// --- Helper: create handler with piped session ---

type testHandlerEnv struct {
	handler    *Handler
	session    *smppsession.Session
	serverConn net.Conn
	clientConn net.Conn
	authMock   *mockAuthClient
}

func newTestHandlerEnv(t *testing.T) *testHandlerEnv {
	t.Helper()
	serverConn, clientConn := net.Pipe()

	logger := newTestLogger()
	sess := smppsession.NewSession(serverConn, logger)
	authMock := &mockAuthClient{}
	authAdapter := NewAuthAdapter(authMock, logger)

	h := NewHandler(sess, authAdapter, nil, nil, logger)

	return &testHandlerEnv{
		handler:    h,
		session:    sess,
		serverConn: serverConn,
		clientConn: clientConn,
		authMock:   authMock,
	}
}

func (e *testHandlerEnv) cleanup() {
	e.serverConn.Close()
	e.clientConn.Close()
}

// readResponse reads from clientConn and parses the response PDU.
func (e *testHandlerEnv) readResponse(t *testing.T) *protocol.PDU {
	t.Helper()
	buf := make([]byte, 4096)
	e.clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := e.clientConn.Read(buf)
	require.NoError(t, err, "failed to read response")
	require.GreaterOrEqual(t, n, protocol.PDUHeaderLength)

	decoder := protocol.NewDecoder(buf[:n])
	pdu, err := decoder.DecodePDU()
	require.NoError(t, err, "failed to decode response PDU")
	return pdu
}

// --- HandlePDU: response PDU pass-through ---

func TestHandlePDU_ResponsePDU(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.EnquireLinkResp, // response bit set
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           nil,
	}

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)
}

// --- HandlePDU: unsupported command ---

func TestHandlePDU_UnsupportedCommand(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.DataSM, // not handled in switch
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           nil,
	}

	// Start reading the generic_nack response in background
	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.GenericNack), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVCMDID), resp.CommandStatus)
}

// --- HandlePDU: enquire_link when bound ---

func TestHandleEnquireLink_Bound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	require.NoError(t, env.session.Bind("transceiver", "sys1", nil, "u1", 100))

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.EnquireLink,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 5,
		Body:           nil,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.EnquireLinkResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_ROK), resp.CommandStatus)
	assert.Equal(t, uint32(5), resp.SequenceNumber)
}

// --- HandlePDU: enquire_link when NOT bound ---

func TestHandleEnquireLink_NotBound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.EnquireLink,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           nil,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.EnquireLinkResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp.CommandStatus)
}

// --- HandlePDU: unbind when not bound ---

func TestHandleUnbind_NotBound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.Unbind,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 2,
		Body:           nil,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.UnbindResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp.CommandStatus)
}

// --- HandlePDU: unbind when bound ---

func TestHandleUnbind_Bound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	require.NoError(t, env.session.Bind("transmitter", "sys1", nil, "u1", 100))

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.Unbind,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 3,
		Body:           nil,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.UnbindResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_ROK), resp.CommandStatus)
	assert.Equal(t, uint32(3), resp.SequenceNumber)
	assert.False(t, env.session.IsBound())
}

// --- HandlePDU: submit_sm when not bound as transmitter ---

func TestHandleSubmitSM_NotBound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	// Session is OPEN, not bound
	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.SubmitSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           []byte{},
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.SubmitSMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp.CommandStatus)
}

// --- HandlePDU: submit_sm bound as receiver (can't send) ---

func TestHandleSubmitSM_BoundReceiver(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	require.NoError(t, env.session.Bind("receiver", "sys1", nil, "u1", 100))

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.SubmitSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           []byte{},
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.SubmitSMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp.CommandStatus)
}

// --- HandlePDU: query_sm when not bound ---

func TestHandleQuerySM_NotBound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.QuerySM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           []byte{},
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.QuerySMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp.CommandStatus)
}

// --- HandlePDU: cancel_sm when not bound ---

func TestHandleCancelSM_NotBound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.CancelSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           []byte{},
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.CancelSMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp.CommandStatus)
}

// --- HandlePDU: replace_sm when not bound ---

func TestHandleReplaceSM_NotBound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.ReplaceSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           []byte{},
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.ReplaceSMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp.CommandStatus)
}

// --- HandlePDU: bind_transceiver success ---

func TestHandleBindTransceiver_Success(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	userUUID := uuid.New().String()

	env.authMock.On("Authenticate", mock.Anything, mock.MatchedBy(func(req *authv1.AuthenticateRequest) bool {
		return req.ApiKey == "test_system"
	})).Return(&authv1.AuthenticateResponse{
		AccessToken: "tok",
		User: &authv1.UserInfo{
			Id:       userUUID,
			Username: "test_user",
			Active:   true,
		},
	}, nil)

	// Encode a bind PDU body
	encoder := protocol.NewEncoder()
	bindBody, err := encoder.EncodeBind(&protocol.BindPDU{
		SystemID:         "test_system",
		Password:         "test_pass",
		SystemType:       "",
		InterfaceVersion: protocol.Version,
		AddrTON:          0,
		AddrNPI:          0,
		AddressRange:     "",
	})
	require.NoError(t, err)

	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(bindBody)),
		CommandID:      protocol.BindTransceiver,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           bindBody,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err = env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.BindTransceiverResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_ROK), resp.CommandStatus)
	assert.Equal(t, uint32(1), resp.SequenceNumber)
	assert.True(t, env.session.IsBound())
	assert.True(t, env.session.CanSend())
	assert.True(t, env.session.CanReceive())
}

// --- HandlePDU: bind_transmitter auth failure ---

func TestHandleBindTransmitter_AuthFailure(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	env.authMock.On("Authenticate", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("invalid credentials"))

	encoder := protocol.NewEncoder()
	bindBody, err := encoder.EncodeBind(&protocol.BindPDU{
		SystemID:         "bad_system",
		Password:         "bad_pass",
		SystemType:       "",
		InterfaceVersion: protocol.Version,
	})
	require.NoError(t, err)

	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(bindBody)),
		CommandID:      protocol.BindTransmitter,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 2,
		Body:           bindBody,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err = env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.BindTransmitterResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVPASWD), resp.CommandStatus)
	assert.False(t, env.session.IsBound())
}

// --- HandlePDU: bind_receiver success ---

func TestHandleBindReceiver_Success(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	userUUID := uuid.New().String()

	env.authMock.On("Authenticate", mock.Anything, mock.Anything).Return(&authv1.AuthenticateResponse{
		AccessToken: "tok",
		User: &authv1.UserInfo{
			Id:       userUUID,
			Username: "rx_user",
			Active:   true,
		},
	}, nil)

	encoder := protocol.NewEncoder()
	bindBody, err := encoder.EncodeBind(&protocol.BindPDU{
		SystemID:         "rx_system",
		Password:         "rx_pass",
		SystemType:       "",
		InterfaceVersion: protocol.Version,
	})
	require.NoError(t, err)

	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(bindBody)),
		CommandID:      protocol.BindReceiver,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           bindBody,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err = env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.BindReceiverResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_ROK), resp.CommandStatus)
	assert.True(t, env.session.IsBound())
	assert.True(t, env.session.CanReceive())
	assert.False(t, env.session.CanSend())
}

// --- HandlePDU: bind with bad body ---

func TestHandleBind_BadBody(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	// Invalid body that cannot be decoded as a bind
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + 2),
		CommandID:      protocol.BindTransceiver,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           []byte{0xFF, 0xFF},
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err := env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.BindTransceiverResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RINVCMDLEN), resp.CommandStatus)
}

// --- HandlePDU: query_sm with nil messageRepo ---

func TestHandleQuerySM_NoMessageRepo(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	require.NoError(t, env.session.Bind("transceiver", "sys1", nil, "u1", 100))

	// Encode query_sm body
	encoder := protocol.NewEncoder()
	queryBody, err := encoder.EncodeQuerySM(&protocol.QuerySMPDU{
		MessageID:     "msg-123",
		SourceAddrTON: 0,
		SourceAddrNPI: 0,
		SourceAddr:    "12345",
	})
	require.NoError(t, err)

	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(queryBody)),
		CommandID:      protocol.QuerySM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 10,
		Body:           queryBody,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err = env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.QuerySMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RQUERYFAIL), resp.CommandStatus)
}

// --- HandlePDU: cancel_sm with nil messageRepo ---

func TestHandleCancelSM_NoMessageRepo(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	require.NoError(t, env.session.Bind("transceiver", "sys1", nil, "u1", 100))

	encoder := protocol.NewEncoder()
	cancelBody, err := encoder.EncodeCancelSM(&protocol.CancelSMPDU{
		ServiceType:     "",
		MessageID:       "msg-123",
		SourceAddrTON:   0,
		SourceAddrNPI:   0,
		SourceAddr:      "12345",
		DestAddrTON:     0,
		DestAddrNPI:     0,
		DestinationAddr: "67890",
	})
	require.NoError(t, err)

	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(cancelBody)),
		CommandID:      protocol.CancelSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 11,
		Body:           cancelBody,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err = env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.CancelSMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RCANCELFAIL), resp.CommandStatus)
}

// --- HandlePDU: replace_sm with nil messageRepo ---

func TestHandleReplaceSM_NoMessageRepo(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	require.NoError(t, env.session.Bind("transceiver", "sys1", nil, "u1", 100))

	msgText := []byte("new text")
	encoder := protocol.NewEncoder()
	replaceBody, err := encoder.EncodeReplaceSM(&protocol.ReplaceSMPDU{
		MessageID:            "msg-123",
		SourceAddrTON:        0,
		SourceAddrNPI:        0,
		SourceAddr:           "12345",
		ScheduleDeliveryTime: "",
		ValidityPeriod:       "",
		RegisteredDelivery:   0,
		SMDefaultMsgID:       0,
		SMLength:             byte(len(msgText)),
		ShortMessage:         msgText,
	})
	require.NoError(t, err)

	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(replaceBody)),
		CommandID:      protocol.ReplaceSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 12,
		Body:           replaceBody,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()

	err = env.handler.HandlePDU(pdu)
	assert.NoError(t, err)

	resp := <-respCh
	assert.Equal(t, uint32(protocol.ReplaceSMResp), resp.CommandID)
	assert.Equal(t, uint32(protocol.ESME_RREPLACEFAIL), resp.CommandStatus)
}

// --- sendPDU with nil connection ---

func TestSendPDU_NilConnection(t *testing.T) {
	logger := newTestLogger()
	conn := newMockServerConn()
	sess := smppsession.NewSession(conn, logger)

	// Close the session so conn becomes nil on GetConn after Close
	sess.Close()

	h := NewHandler(sess, nil, nil, nil, logger)

	pdu := &protocol.PDU{
		CommandLength:  protocol.PDUHeaderLength,
		CommandID:      protocol.EnquireLinkResp,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           nil,
	}

	// The connection should be nil since we set Conn to nil in the session after close
	// Actually Close() doesn't nil Conn, but the state is CLOSED. Let's nil it manually.
	sess.Conn = nil

	err := h.sendPDU(pdu)
	assert.ErrorIs(t, err, smppsession.ErrConnectionClosed)
}

// --- authenticate with nil authAdapter ---

func TestAuthenticate_NilAuthAdapter(t *testing.T) {
	logger := newTestLogger()
	conn := newMockServerConn()
	sess := smppsession.NewSession(conn, logger)

	h := NewHandler(sess, nil, nil, nil, logger)

	_, err := h.authenticate("sys", "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "аутентификация недоступна")
}

// --- gwMapMessageStatusToSMPP ---

func TestGwMapMessageStatusToSMPP(t *testing.T) {
	tests := []struct {
		status   shared.MessageStatus
		expected byte
	}{
		{shared.MessageStatusPending, protocol.MSG_STATE_ENROUTE},
		{shared.MessageStatusQueued, protocol.MSG_STATE_ENROUTE},
		{shared.MessageStatusSent, protocol.MSG_STATE_ACCEPTED},
		{shared.MessageStatusDelivered, protocol.MSG_STATE_DELIVERED},
		{shared.MessageStatusExpired, protocol.MSG_STATE_EXPIRED},
		{shared.MessageStatusFailed, protocol.MSG_STATE_UNDELIVERABLE},
		{shared.MessageStatusRejected, protocol.MSG_STATE_REJECTED},
		{shared.MessageStatusCancelled, protocol.MSG_STATE_DELETED},
		{shared.MessageStatusScheduled, protocol.MSG_STATE_SCHEDULED},
		{shared.MessageStatus("unknown"), protocol.MSG_STATE_UNKNOWN},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := gwMapMessageStatusToSMPP(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- NewHandler ---

func TestNewHandler(t *testing.T) {
	logger := newTestLogger()
	conn := newMockServerConn()
	sess := smppsession.NewSession(conn, logger)
	authMock := &mockAuthClient{}
	authAdapter := NewAuthAdapter(authMock, logger)

	h := NewHandler(sess, authAdapter, nil, nil, logger)

	require.NotNil(t, h)
	assert.Equal(t, sess, h.session)
	assert.NotNil(t, h.decoder)
	assert.NotNil(t, h.encoder)
	assert.NotNil(t, h.validator)
	assert.Equal(t, authAdapter, h.authAdapter)
}

// --- Double bind attempt ---

func TestHandleBind_AlreadyBound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup()

	userUUID := uuid.New().String()

	env.authMock.On("Authenticate", mock.Anything, mock.Anything).Return(&authv1.AuthenticateResponse{
		AccessToken: "tok",
		User: &authv1.UserInfo{
			Id:       userUUID,
			Username: "test_user",
			Active:   true,
		},
	}, nil)

	encoder := protocol.NewEncoder()
	bindBody, err := encoder.EncodeBind(&protocol.BindPDU{
		SystemID:         "sys1",
		Password:         "pass1",
		InterfaceVersion: protocol.Version,
	})
	require.NoError(t, err)

	// First bind - should succeed
	pdu1 := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(bindBody)),
		CommandID:      protocol.BindTransceiver,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
		Body:           bindBody,
	}

	respCh := make(chan *protocol.PDU, 1)
	go func() {
		respCh <- env.readResponse(t)
	}()
	err = env.handler.HandlePDU(pdu1)
	assert.NoError(t, err)
	resp1 := <-respCh
	assert.Equal(t, uint32(protocol.ESME_ROK), resp1.CommandStatus)

	// Second bind - should fail (already bound)
	pdu2 := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(bindBody)),
		CommandID:      protocol.BindTransceiver,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 2,
		Body:           bindBody,
	}

	go func() {
		respCh <- env.readResponse(t)
	}()
	err = env.handler.HandlePDU(pdu2)
	assert.NoError(t, err)
	resp2 := <-respCh
	assert.Equal(t, uint32(protocol.ESME_RINVBNDSTS), resp2.CommandStatus)
}
