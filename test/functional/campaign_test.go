//go:build functional

package functional_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/campaign/application"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
	"github.com/smpp-server/smpp-server/internal/services/campaign/infrastructure/repository"
)

func TestCampaignChain(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	contactListID := uuid.New()

	ctx := context.Background()

	// Insert prerequisite client row.
	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-campaign', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	// Insert prerequisite contact list row.
	_, err = db.ExecContext(ctx,
		`INSERT INTO contact_lists (id, client_id, name, contacts_count)
		 VALUES ($1, $2, 'test-list', 0)
		 ON CONFLICT DO NOTHING`, contactListID.String(), clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaign_ab_config WHERE campaign_id IN (SELECT id FROM campaigns WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaign_variants WHERE campaign_id IN (SELECT id FROM campaigns WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaigns WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_lists WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	campaignRepo := repository.NewCampaignRepository(db)
	recipientRepo := repository.NewRecipientRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	svc := application.NewCampaignService(campaignRepo, recipientRepo, statsRepo)

	t.Run("CreateAndGetCampaign", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "Test Campaign",
			contactListID.String(), "", "TestSender", "", nil, 100, nil, false)
		require.NoError(t, err)
		require.NotNil(t, campaign)

		assert.Equal(t, "Test Campaign", campaign.Name)
		assert.Equal(t, domain.StatusDraft, campaign.Status)
		assert.Equal(t, clientID, campaign.ClientID)
		assert.Equal(t, contactListID, campaign.ContactListID)
		assert.Equal(t, "TestSender", campaign.Source)
		assert.Equal(t, int32(100), campaign.SendRate)

		// Retrieve the campaign.
		fetched, err := svc.GetCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, campaign.ID, fetched.ID)
		assert.Equal(t, "Test Campaign", fetched.Name)
		assert.Equal(t, domain.StatusDraft, fetched.Status)
	})

	t.Run("CreateCampaignEmptyNameFails", func(t *testing.T) {
		_, err := svc.CreateCampaign(ctx, clientID, "",
			contactListID.String(), "", "Sender", "", nil, 100, nil, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("ListCampaigns", func(t *testing.T) {
		// Create two campaigns.
		_, err := svc.CreateCampaign(ctx, clientID, "List Campaign A",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		_, err = svc.CreateCampaign(ctx, clientID, "List Campaign B",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		campaigns, total, err := svc.ListCampaigns(ctx, clientID, "", 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(campaigns), 2)

		// Filter by status.
		campaigns, total, err = svc.ListCampaigns(ctx, clientID, domain.StatusDraft, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		for _, c := range campaigns {
			assert.Equal(t, domain.StatusDraft, c.Status)
		}
	})

	t.Run("UpdateCampaign", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "To Update",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		updated, err := svc.UpdateCampaign(ctx, campaign.ID, clientID,
			"Updated Name", "", "", "NewSender", "", nil, 200, nil)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "NewSender", updated.Source)
		assert.Equal(t, int32(200), updated.SendRate)
	})

	t.Run("DeleteCampaignDraft", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "To Delete",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		err = svc.DeleteCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)

		_, err = svc.GetCampaign(ctx, campaign.ID, clientID)
		assert.ErrorIs(t, err, domain.ErrCampaignNotFound)
	})

	t.Run("LaunchCampaign", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "To Launch",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusDraft, campaign.Status)

		launched, err := svc.LaunchCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusMaterializing, launched.Status)
		assert.NotNil(t, launched.StartedAt)

		// Launching a non-draft campaign must fail.
		_, err = svc.LaunchCampaign(ctx, campaign.ID, clientID)
		assert.ErrorIs(t, err, domain.ErrCampaignNotDraft)
	})

	t.Run("ScheduledCampaign", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		campaign, err := svc.CreateCampaign(ctx, clientID, "Scheduled",
			contactListID.String(), "", "Sender", "", nil, 50, &future, false)
		require.NoError(t, err)
		require.NotNil(t, campaign.ScheduledAt)
		assert.WithinDuration(t, future, *campaign.ScheduledAt, time.Second)

		// Can still launch a draft with schedule.
		launched, err := svc.LaunchCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusMaterializing, launched.Status)
	})

	t.Run("CancelCampaign", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "To Cancel",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		// Launch to materializing.
		launched, err := svc.LaunchCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusMaterializing, launched.Status)

		// Cancel from materializing.
		cancelled, err := svc.CancelCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusCancelled, cancelled.Status)
		assert.NotNil(t, cancelled.CompletedAt)
	})

	t.Run("CancelDraftFails", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "Draft Cancel Fail",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		_, err = svc.CancelCampaign(ctx, campaign.ID, clientID)
		assert.ErrorIs(t, err, domain.ErrInvalidCampaignStatus)
	})

	t.Run("UpdateNonDraftFails", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "No Update After Launch",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		_, err = svc.LaunchCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)

		_, err = svc.UpdateCampaign(ctx, campaign.ID, clientID,
			"Should Fail", "", "", "", "", nil, 0, nil)
		assert.ErrorIs(t, err, domain.ErrCampaignNotDraft)
	})

	t.Run("CampaignNotFoundForOtherClient", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "Ownership Check",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		otherClient := uuid.New()
		_, err = svc.GetCampaign(ctx, campaign.ID, otherClient)
		assert.ErrorIs(t, err, domain.ErrCampaignNotFound)
	})
}

func TestCampaignABTesting(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	contactListID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-ab', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO contact_lists (id, client_id, name, contacts_count)
		 VALUES ($1, $2, 'test-ab-list', 0)
		 ON CONFLICT DO NOTHING`, contactListID.String(), clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaign_ab_config WHERE campaign_id IN (SELECT id FROM campaigns WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaign_variants WHERE campaign_id IN (SELECT id FROM campaigns WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaigns WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_lists WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	campaignRepo := repository.NewCampaignRepository(db)
	recipientRepo := repository.NewRecipientRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	svc := application.NewCampaignService(campaignRepo, recipientRepo, statsRepo)

	t.Run("SetVariants", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "AB Test Campaign",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		variants := []domain.Variant{
			{Name: "Variant A", Percentage: 50, IsControl: true},
			{Name: "Variant B", Percentage: 50},
		}

		result, err := svc.SetVariants(ctx, campaign.ID, clientID, variants)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "Variant A", result[0].Name)
		assert.Equal(t, int32(50), result[0].Percentage)
		assert.True(t, result[0].IsControl)
	})

	t.Run("VariantPercentageMustSum100", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "Bad Percentage",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		variants := []domain.Variant{
			{Name: "A", Percentage: 30},
			{Name: "B", Percentage: 30},
		}

		_, err = svc.SetVariants(ctx, campaign.ID, clientID, variants)
		assert.ErrorIs(t, err, domain.ErrVariantPercentageSum)
	})

	t.Run("TooFewVariantsFails", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "One Variant",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		variants := []domain.Variant{
			{Name: "A", Percentage: 100},
		}

		_, err = svc.SetVariants(ctx, campaign.ID, clientID, variants)
		assert.ErrorIs(t, err, domain.ErrTooFewVariants)
	})

	t.Run("SetABConfigAndSelectWinner", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "AB Config Test",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		variants := []domain.Variant{
			{Name: "Control", Percentage: 50, IsControl: true},
			{Name: "Challenger", Percentage: 50},
		}
		createdVariants, err := svc.SetVariants(ctx, campaign.ID, clientID, variants)
		require.NoError(t, err)

		abConfig, err := svc.SetABConfig(ctx, campaign.ID, clientID, "delivery_rate", 24, true)
		require.NoError(t, err)
		assert.Equal(t, "delivery_rate", abConfig.Metric)
		assert.Equal(t, int32(24), abConfig.TestDurationHours)
		assert.True(t, abConfig.AutoSelectWinner)

		// Select a winner.
		winnerID := createdVariants[0].ID
		result, err := svc.SelectWinner(ctx, campaign.ID, clientID, winnerID)
		require.NoError(t, err)
		require.NotNil(t, result.ABConfig)
		require.NotNil(t, result.ABConfig.WinnerVariantID)
		assert.Equal(t, winnerID, *result.ABConfig.WinnerVariantID)

		// Selecting winner again must fail.
		_, err = svc.SelectWinner(ctx, campaign.ID, clientID, createdVariants[1].ID)
		assert.ErrorIs(t, err, domain.ErrWinnerAlreadySelected)
	})
}

func TestCampaignRetryConfig(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	contactListID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-retry', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO contact_lists (id, client_id, name, contacts_count)
		 VALUES ($1, $2, 'test-retry-list', 0)
		 ON CONFLICT DO NOTHING`, contactListID.String(), clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaign_ab_config WHERE campaign_id IN (SELECT id FROM campaigns WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaign_variants WHERE campaign_id IN (SELECT id FROM campaigns WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM campaigns WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_lists WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	campaignRepo := repository.NewCampaignRepository(db)
	recipientRepo := repository.NewRecipientRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	svc := application.NewCampaignService(campaignRepo, recipientRepo, statsRepo)

	t.Run("SetAndGetRetryConfig", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "Retry Test",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		rc := &domain.RetryConfig{
			Enabled:    true,
			DelayHours: 2,
			MaxRetries: 3,
		}

		retryConfig, err := svc.SetRetryConfig(ctx, campaign.ID, clientID, rc)
		require.NoError(t, err)
		assert.True(t, retryConfig.Enabled)
		assert.Equal(t, int32(2), retryConfig.DelayHours)
		assert.Equal(t, int32(3), retryConfig.MaxRetries)

		// Verify via get campaign.
		fetched, err := svc.GetCampaign(ctx, campaign.ID, clientID)
		require.NoError(t, err)
		require.NotNil(t, fetched.RetryConfig)
		assert.True(t, fetched.RetryConfig.Enabled)
		assert.Equal(t, int32(3), fetched.RetryConfig.MaxRetries)
	})

	t.Run("RetryFailedWithNoRecipientsFails", func(t *testing.T) {
		campaign, err := svc.CreateCampaign(ctx, clientID, "No Failed",
			contactListID.String(), "", "Sender", "", nil, 50, nil, false)
		require.NoError(t, err)

		_, err = svc.RetryFailed(ctx, campaign.ID, clientID, "")
		assert.ErrorIs(t, err, domain.ErrNoFailedRecipients)
	})
}
