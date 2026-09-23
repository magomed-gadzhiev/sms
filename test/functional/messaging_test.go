//go:build functional

package functional_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/messaging/application"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
	msgRepo "github.com/smpp-server/smpp-server/internal/services/messaging/infrastructure/repository"
	msgMocks "github.com/smpp-server/smpp-server/internal/services/messaging/mocks"
	"github.com/smpp-server/smpp-server/internal/shared"
)

func TestMessagingChain(t *testing.T) {
	skipIfNoDB(t)
	db := setupTestDB(t)
	ensurePartition(t, db)

	t.Run("SendMessage", func(t *testing.T) {
		ctx := context.Background()
		clientID := uuid.New()

		messageRepo := msgRepo.NewMessageRepository(db)
		dlrRepo := msgRepo.NewDLRRepository(db)

		mockPublisher := &msgMocks.MockEventPublisher{}
		mockPublisher.On("PublishMessageQueued", mock.Anything, mock.Anything).Return(nil)

		svc := application.NewMessageService(messageRepo, dlrRepo, mockPublisher)

		msg, err := svc.SendMessage(ctx, clientID, "TestSrc", "+79001234567", "Hello functional test", nil)
		require.NoError(t, err)
		require.NotNil(t, msg)

		// Immediate sends are publish-only: the persist stage batch-inserts
		// into the DB asynchronously (COPY), so there is no row to read back
		// here. The seam's contract: the returned message is queued and the
		// queued event was published.
		assert.Equal(t, shared.MessageStatusQueued, msg.Status)
		assert.Equal(t, "+79001234567", msg.Destination)
		assert.Equal(t, "Hello functional test", msg.Text)

		mockPublisher.AssertCalled(t, "PublishMessageQueued", mock.Anything, mock.Anything)
	})

	t.Run("ProcessDLR", func(t *testing.T) {
		ctx := context.Background()
		clientID := uuid.New()

		_, err := db.ExecContext(ctx,
			`INSERT INTO clients (id, name, api_key, secret)
			 VALUES ($1, 'test-dlr', $1, 'secret')
			 ON CONFLICT DO NOTHING`, clientID.String())
		require.NoError(t, err)

		messageRepo := msgRepo.NewMessageRepository(db)
		dlrRepo := msgRepo.NewDLRRepository(db)

		// Insert a message in SENT status directly
		msgID := uuid.New()
		smppMsgID := "SMPP-DLR-" + uuid.New().String()[:8]
		now := time.Now()
		msg := &domain.Message{
			ID:                 msgID,
			Source:             "TestSrc",
			Destination:        "+79001234568",
			Text:               "DLR test message",
			Encoding:           shared.MessageEncodingGSM7,
			Status:             shared.MessageStatusSent,
			ClientID:           &clientID,
			SMPPMessageID:      smppMsgID,
			RegisteredDelivery: 1,
			MaxRetries:         5,
			SegmentCount:       1,
			SubmittedAt:        &now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		err = messageRepo.Create(ctx, msg)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM dlr_receipts WHERE message_id = $1`, msgID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM messages WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM clients WHERE id = $1`, clientID)
		})

		// Process DLR with DELIVRD status
		mockPublisher := &msgMocks.MockEventPublisher{}
		mockPublisher.On("PublishMessageStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		dlrSvc := application.NewDLRService(messageRepo, dlrRepo, mockPublisher)
		doneDate := time.Now()
		err = dlrSvc.ProcessDLR(ctx, msgID, smppMsgID, "DELIVRD", &doneDate, nil, "", nil)
		require.NoError(t, err)

		// Verify message status is now DELIVERED
		fetched, err := messageRepo.GetByID(ctx, msgID)
		require.NoError(t, err)
		assert.Equal(t, shared.MessageStatusDelivered, fetched.Status)
		assert.NotNil(t, fetched.DeliveredAt)

		// Verify DLR receipt row exists
		dlrs, err := dlrRepo.GetByMessageID(ctx, msgID)
		require.NoError(t, err)
		require.Len(t, dlrs, 1)
		assert.Equal(t, "DELIVRD", dlrs[0].Stat)
		assert.Equal(t, smppMsgID, dlrs[0].SMPPMessageID)
	})

	t.Run("CancelMessage", func(t *testing.T) {
		ctx := context.Background()
		clientID := uuid.New()

		_, err := db.ExecContext(ctx,
			`INSERT INTO clients (id, name, api_key, secret)
			 VALUES ($1, 'test-cancel', $1, 'secret')
			 ON CONFLICT DO NOTHING`, clientID.String())
		require.NoError(t, err)

		messageRepo := msgRepo.NewMessageRepository(db)
		dlrRepo := msgRepo.NewDLRRepository(db)

		// Insert a SCHEDULED message
		msgID := uuid.New()
		now := time.Now()
		scheduledAt := now.Add(1 * time.Hour)
		msg := &domain.Message{
			ID:                 msgID,
			Source:             "TestSrc",
			Destination:        "+79001234569",
			Text:               "Cancel test message",
			Encoding:           shared.MessageEncodingGSM7,
			Status:             shared.MessageStatusScheduled,
			ClientID:           &clientID,
			RegisteredDelivery: 1,
			MaxRetries:         5,
			SegmentCount:       1,
			ScheduledAt:        &scheduledAt,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		err = messageRepo.Create(ctx, msg)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM messages WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM clients WHERE id = $1`, clientID)
		})

		mockPublisher := &msgMocks.MockEventPublisher{}
		svc := application.NewMessageService(messageRepo, dlrRepo, mockPublisher)

		// Cancel the scheduled message
		err = svc.CancelMessage(ctx, msgID, clientID)
		require.NoError(t, err)

		// Verify status is now CANCELLED
		fetched, err := messageRepo.GetByID(ctx, msgID)
		require.NoError(t, err)
		assert.Equal(t, shared.MessageStatusCancelled, fetched.Status)
	})
}
