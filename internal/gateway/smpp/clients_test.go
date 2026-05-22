package smpp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceClientsClose_Empty(t *testing.T) {
	clients := &ServiceClients{}

	err := clients.Close()
	require.NoError(t, err)
}

func TestWithAuthToken(t *testing.T) {
	ctx := context.Background()
	ctx = WithAuthToken(ctx, "my-token-123")

	token, ok := GetAuthToken(ctx)
	assert.True(t, ok)
	assert.Equal(t, "my-token-123", token)
}

func TestGetAuthToken_Missing(t *testing.T) {
	ctx := context.Background()

	token, ok := GetAuthToken(ctx)
	assert.False(t, ok)
	assert.Empty(t, token)
}

func TestWithAuthToken_EmptyToken(t *testing.T) {
	ctx := context.Background()
	ctx = WithAuthToken(ctx, "")

	token, ok := GetAuthToken(ctx)
	// An empty string is still a valid string, but it's falsy for the empty check
	// The implementation uses type assertion, empty string would succeed the assertion
	assert.True(t, ok)
	assert.Empty(t, token)
}

func TestWithAuthToken_OverwriteToken(t *testing.T) {
	ctx := context.Background()
	ctx = WithAuthToken(ctx, "first-token")
	ctx = WithAuthToken(ctx, "second-token")

	token, ok := GetAuthToken(ctx)
	assert.True(t, ok)
	assert.Equal(t, "second-token", token)
}

func TestServiceAddresses(t *testing.T) {
	addrs := ServiceAddresses{
		Auth: "localhost:9090",
	}

	assert.Equal(t, "localhost:9090", addrs.Auth)
}

func TestNewServiceClients_EmptyAuthAddress(t *testing.T) {
	// When Auth address is empty, no connection is attempted
	clients, err := NewServiceClients(ServiceAddresses{Auth: ""})
	require.NoError(t, err)
	require.NotNil(t, clients)
	assert.Nil(t, clients.AuthClient)
	assert.Empty(t, clients.conns)

	require.NoError(t, clients.Close())
}
