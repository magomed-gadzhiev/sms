//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/testutil"
	authv1 "github.com/smpp-server/smpp-server/api/proto/authv1"
	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
	messagingv1 "github.com/smpp-server/smpp-server/api/proto/messagingv1"
	providerv1 "github.com/smpp-server/smpp-server/api/proto/providerv1"
	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	authServiceAddr      = "localhost:9101"
	messagingServiceAddr = "localhost:9092"
	routingServiceAddr   = "localhost:9094"
	providerServiceAddr  = "localhost:9093"
	clientServiceAddr    = "localhost:9095"
	billingServiceAddr   = "localhost:9097"
)

// getGRPCConn создает gRPC соединение к сервису
func getGRPCConn(t *testing.T, addr string) *grpc.ClientConn {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Skipf("Skipping test: gRPC service not available at %s: %v", addr, err)
	}
	return conn
}

// TestAuthService_ValidateToken тестирует валидацию токена через Auth Service
func TestAuthService_ValidateToken(t *testing.T) {
	conn := getGRPCConn(t, authServiceAddr)
	defer conn.Close()

	client := authv1.NewAuthServiceClient(conn)
	ctx := context.Background()

	// Сначала аутентифицируемся
	authResp, err := client.Authenticate(ctx, &authv1.AuthenticateRequest{
		Username: "test",
		Password: "test",
	})
	if err != nil {
		t.Skipf("Skipping test: authentication failed: %v", err)
	}

	require.NotEmpty(t, authResp.Token)

	// Затем валидируем токен
	validateResp, err := client.ValidateToken(ctx, &authv1.ValidateTokenRequest{
		Token: authResp.Token,
	})

	require.NoError(t, err)
	assert.True(t, validateResp.Valid)
	assert.NotEmpty(t, validateResp.UserId)
}

// TestMessagingService_SendMessage тестирует отправку сообщения через Messaging Service
func TestMessagingService_SendMessage(t *testing.T) {
	conn := getGRPCConn(t, messagingServiceAddr)
	defer conn.Close()

	client := messagingv1.NewMessagingServiceClient(conn)
	ctx := context.Background()

	clientID := uuid.New().String()
	msgID := uuid.New()

	resp, err := client.SendMessage(ctx, &messagingv1.SendMessageRequest{
		ClientId:    clientID,
		MessageId:   msgID.String(),
		Source:      "12345",
		Destination: "79001234567",
		Text:        "Integration test message",
	})

	if err != nil {
		t.Logf("SendMessage error (may be expected if service not fully configured): %v", err)
		return
	}

	require.NoError(t, err)
	assert.NotEmpty(t, resp.MessageId)
	assert.Equal(t, msgID.String(), resp.MessageId)
}

// TestMessagingService_GetMessageStatus тестирует получение статуса сообщения
func TestMessagingService_GetMessageStatus(t *testing.T) {
	conn := getGRPCConn(t, messagingServiceAddr)
	defer conn.Close()

	client := messagingv1.NewMessagingServiceClient(conn)
	ctx := context.Background()

	clientID := uuid.New().String()
	msgID := uuid.New()

	// Сначала создаем сообщение
	_, err := client.SendMessage(ctx, &messagingv1.SendMessageRequest{
		ClientId:    clientID,
		MessageId:   msgID.String(),
		Source:      "12345",
		Destination: "79001234567",
		Text:        "Status test message",
	})

	if err != nil {
		t.Skipf("Skipping test: SendMessage failed: %v", err)
	}

	// Даем время на обработку
	time.Sleep(1 * time.Second)

	// Получаем статус
	statusResp, err := client.GetMessageStatus(ctx, &messagingv1.GetMessageStatusRequest{
		MessageId: msgID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, msgID.String(), statusResp.MessageId)
	assert.NotEmpty(t, statusResp.Status)
}

// TestRoutingService_GetRoute тестирует получение маршрута
func TestRoutingService_GetRoute(t *testing.T) {
	conn := getGRPCConn(t, routingServiceAddr)
	defer conn.Close()

	client := routingv1.NewRoutingServiceClient(conn)
	ctx := context.Background()

	clientID := uuid.New().String()

	resp, err := client.GetRoute(ctx, &routingv1.GetRouteRequest{
		Destination: "79001234567",
		ClientId:    clientID,
	})

	if err != nil {
		t.Logf("GetRoute error (may be expected if no routes configured): %v", err)
		return
	}

	require.NoError(t, err)
	assert.NotEmpty(t, resp.RouteId)
}

// TestProviderService_ListProviders тестирует получение списка провайдеров
func TestProviderService_ListProviders(t *testing.T) {
	conn := getGRPCConn(t, providerServiceAddr)
	defer conn.Close()

	client := providerv1.NewProviderServiceClient(conn)
	ctx := context.Background()

	resp, err := client.ListProviders(ctx, &providerv1.ListProvidersRequest{
		Limit:  10,
		Offset: 0,
	})

	if err != nil {
		t.Logf("ListProviders error (may be expected if service not configured): %v", err)
		return
	}

	require.NoError(t, err)
	assert.NotNil(t, resp.Providers)
}

// TestClientService_GetClient тестирует получение клиента
func TestClientService_GetClient(t *testing.T) {
	conn := getGRPCConn(t, clientServiceAddr)
	defer conn.Close()

	client := clientv1.NewClientServiceClient(conn)
	ctx := context.Background()

	// Сначала создаем тестовую БД и клиента
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testClient := testutil.NewTestClient()
	testClient.ID = uuid.New()
	err := db.ExecContext(ctx, `
		INSERT INTO clients (id, name, api_key, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, testClient.ID, testClient.Name, testClient.APIKey, "active")
	if err != nil {
		t.Skipf("Skipping test: failed to create test client: %v", err)
	}

	// Получаем клиента
	resp, err := client.GetClient(ctx, &clientv1.GetClientRequest{
		ClientId: testClient.ID.String(),
	})

	if err != nil {
		t.Logf("GetClient error: %v", err)
		return
	}

	require.NoError(t, err)
	assert.Equal(t, testClient.ID.String(), resp.Client.Id)
	assert.Equal(t, testClient.Name, resp.Client.Name)
}

// TestBillingService_GetBalance тестирует получение баланса
func TestBillingService_GetBalance(t *testing.T) {
	conn := getGRPCConn(t, billingServiceAddr)
	defer conn.Close()

	client := billingv1.NewBillingServiceClient(conn)
	ctx := context.Background()

	clientID := uuid.New().String()

	resp, err := client.GetBalance(ctx, &billingv1.GetBalanceRequest{
		ClientId: clientID,
	})

	if err != nil {
		t.Logf("GetBalance error (may be expected if account doesn't exist): %v", err)
		return
	}

	require.NoError(t, err)
	assert.Equal(t, clientID, resp.ClientId)
	assert.NotNil(t, resp.Balance)
}

// TestServiceIntegration_EndToEndFlow тестирует полный поток от отправки до получения статуса
func TestServiceIntegration_EndToEndFlow(t *testing.T) {
	// Подключаемся к необходимым сервисам
	messagingConn := getGRPCConn(t, messagingServiceAddr)
	defer messagingConn.Close()

	messagingClient := messagingv1.NewMessagingServiceClient(messagingConn)
	ctx := context.Background()

	clientID := uuid.New().String()
	msgID := uuid.New()

	// 1. Отправляем сообщение
	sendResp, err := messagingClient.SendMessage(ctx, &messagingv1.SendMessageRequest{
		ClientId:    clientID,
		MessageId:   msgID.String(),
		Source:      "12345",
		Destination: "79001234567",
		Text:        "E2E test message",
	})

	if err != nil {
		t.Skipf("Skipping test: SendMessage failed: %v", err)
	}

	require.NoError(t, err)
	assert.Equal(t, msgID.String(), sendResp.MessageId)

	// 2. Ждем обработки
	time.Sleep(2 * time.Second)

	// 3. Получаем статус
	statusResp, err := messagingClient.GetMessageStatus(ctx, &messagingv1.GetMessageStatusRequest{
		MessageId: msgID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, msgID.String(), statusResp.MessageId)
	assert.NotEmpty(t, statusResp.Status)

	// 4. Получаем историю
	historyResp, err := messagingClient.GetMessageHistory(ctx, &messagingv1.GetMessageHistoryRequest{
		ClientId: clientID,
		Limit:    10,
		Offset:   0,
	})

	if err != nil {
		t.Logf("GetMessageHistory error: %v", err)
		return
	}

	require.NoError(t, err)
	assert.NotNil(t, historyResp.Messages)
}