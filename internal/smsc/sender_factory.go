package smsc

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// AsyncSender — интерфейс async-отправки, общий для SMPP и Stub.
type AsyncSender interface {
	SendMessageAsync(ctx context.Context, msg *shared.Message, provider *shared.Provider, conn *AsyncConnection) (string, error)
}

// SenderFactory выбирает реализацию AsyncSender по типу провайдера.
type SenderFactory struct {
	smpp *Sender
	stub *StubSender
}

func NewSenderFactory(smpp *Sender, stub *StubSender) *SenderFactory {
	return &SenderFactory{smpp: smpp, stub: stub}
}

// For возвращает StubSender для SIMULATOR-провайдеров, иначе — Sender (SMPP).
func (f *SenderFactory) For(provider *shared.Provider) AsyncSender {
	if IsSimulator(provider) {
		return f.stub
	}
	return f.smpp
}
