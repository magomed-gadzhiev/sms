package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// validProvider returns a Provider that passes all validation rules.
func validProvider() *Provider {
	return &Provider{
		Name:           "Test Provider",
		Host:           "smpp.example.com",
		Port:           2775,
		SystemID:       "sys1",
		Password:       "secret",
		MaxConnections: 5,
		BindType:       BindTypeTransceiver,
	}
}

// --- Validate ---

func TestValidate_ValidProvider(t *testing.T) {
	p := validProvider()
	assert.NoError(t, p.Validate())
}

func TestValidate_EmptyName(t *testing.T) {
	p := validProvider()
	p.Name = ""
	assert.ErrorIs(t, p.Validate(), ErrProviderNameRequired)
}

func TestValidate_EmptyHost(t *testing.T) {
	p := validProvider()
	p.Host = ""
	assert.ErrorIs(t, p.Validate(), ErrProviderHostRequired)
}

func TestValidate_PortZero(t *testing.T) {
	p := validProvider()
	p.Port = 0
	assert.ErrorIs(t, p.Validate(), ErrProviderPortInvalid)
}

func TestValidate_PortNegative(t *testing.T) {
	p := validProvider()
	p.Port = -1
	assert.ErrorIs(t, p.Validate(), ErrProviderPortInvalid)
}

func TestValidate_PortAboveMax(t *testing.T) {
	p := validProvider()
	p.Port = 65536
	assert.ErrorIs(t, p.Validate(), ErrProviderPortInvalid)
}

func TestValidate_PortAtMin(t *testing.T) {
	p := validProvider()
	p.Port = 1
	assert.NoError(t, p.Validate())
}

func TestValidate_PortAtMax(t *testing.T) {
	p := validProvider()
	p.Port = 65535
	assert.NoError(t, p.Validate())
}

func TestValidate_EmptySystemID(t *testing.T) {
	p := validProvider()
	p.SystemID = ""
	assert.ErrorIs(t, p.Validate(), ErrProviderSystemIDRequired)
}

func TestValidate_EmptyPassword(t *testing.T) {
	p := validProvider()
	p.Password = ""
	assert.ErrorIs(t, p.Validate(), ErrProviderPasswordRequired)
}

func TestValidate_MaxConnectionsNegative(t *testing.T) {
	p := validProvider()
	p.MaxConnections = -1
	assert.ErrorIs(t, p.Validate(), ErrProviderMaxConnectionsInvalid)
}

func TestValidate_MaxConnectionsZeroOK(t *testing.T) {
	p := validProvider()
	p.MaxConnections = 0
	assert.NoError(t, p.Validate())
}

func TestValidate_InvalidBindType(t *testing.T) {
	p := validProvider()
	p.BindType = "invalid"
	assert.ErrorIs(t, p.Validate(), ErrProviderBindTypeInvalid)
}

func TestValidate_EmptyBindType(t *testing.T) {
	p := validProvider()
	p.BindType = ""
	assert.ErrorIs(t, p.Validate(), ErrProviderBindTypeInvalid)
}

func TestValidate_BindTypeTransmitter(t *testing.T) {
	p := validProvider()
	p.BindType = BindTypeTransmitter
	assert.NoError(t, p.Validate())
}

func TestValidate_BindTypeReceiver(t *testing.T) {
	p := validProvider()
	p.BindType = BindTypeReceiver
	assert.NoError(t, p.Validate())
}

// --- IsValidBindType ---

func TestIsValidBindType_Transceiver(t *testing.T) {
	p := &Provider{BindType: BindTypeTransceiver}
	assert.True(t, p.IsValidBindType())
}

func TestIsValidBindType_Transmitter(t *testing.T) {
	p := &Provider{BindType: BindTypeTransmitter}
	assert.True(t, p.IsValidBindType())
}

func TestIsValidBindType_Receiver(t *testing.T) {
	p := &Provider{BindType: BindTypeReceiver}
	assert.True(t, p.IsValidBindType())
}

func TestIsValidBindType_Invalid(t *testing.T) {
	p := &Provider{BindType: "unknown"}
	assert.False(t, p.IsValidBindType())
}

func TestIsValidBindType_Empty(t *testing.T) {
	p := &Provider{BindType: ""}
	assert.False(t, p.IsValidBindType())
}

// --- IsActive ---

func TestIsActive_True(t *testing.T) {
	p := &Provider{Active: true}
	assert.True(t, p.IsActive())
}

func TestIsActive_False(t *testing.T) {
	p := &Provider{Active: false}
	assert.False(t, p.IsActive())
}

// --- Errors are distinct ---

func TestErrors_Distinct(t *testing.T) {
	errs := []error{
		ErrProviderNotFound,
		ErrProviderLimitExceeded,
		ErrProviderNotOwnedByClient,
		ErrProviderNameRequired,
		ErrProviderHostRequired,
		ErrProviderPortInvalid,
		ErrProviderSystemIDRequired,
		ErrProviderPasswordRequired,
		ErrProviderMaxConnectionsInvalid,
		ErrProviderBindTypeInvalid,
		ErrConnectionNotFound,
		ErrConnectionNotBound,
		ErrConnectionPoolFull,
		ErrProviderInactive,
	}

	seen := make(map[error]bool)
	for _, e := range errs {
		assert.NotNil(t, e)
		assert.False(t, seen[e], "duplicate error sentinel: %v", e)
		seen[e] = true
	}
}
