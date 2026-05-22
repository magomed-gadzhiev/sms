package smsc_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
)

func TestSenderFactory_ReturnsStubSenderForSimulator(t *testing.T) {
	factory := smsc.NewSenderFactory(nil, smsc.NewStubSender(nil, nil, ""))
	provider := &shared.Provider{ID: uuid.New(), SystemType: "SIMULATOR"}
	sender := factory.For(provider)
	assert.IsType(t, &smsc.StubSender{}, sender)
}

func TestSenderFactory_ReturnsSMPPSenderForReal(t *testing.T) {
	smppSender := smsc.NewSender(nil)
	factory := smsc.NewSenderFactory(smppSender, smsc.NewStubSender(nil, nil, ""))
	provider := &shared.Provider{ID: uuid.New(), SystemType: "SMPP"}
	sender := factory.For(provider)
	assert.IsType(t, &smsc.Sender{}, sender)
}
