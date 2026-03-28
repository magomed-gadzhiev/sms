package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	"github.com/smpp-server/smpp-server/internal/services/campaign/application"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
	"github.com/smpp-server/smpp-server/internal/services/campaign/mocks"
)

// helpers

func newServer(svc *mocks.MockCampaignServicer) *Server {
	return NewServer(svc)
}

func newCampaign(clientID uuid.UUID) *domain.Campaign {
	return &domain.Campaign{
		ID:            uuid.New(),
		ClientID:      clientID,
		Name:          "Test Campaign",
		Status:        domain.StatusDraft,
		ContactListID: uuid.New(),
		SegmentTags:   []string{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// ---- CreateCampaign ----

func TestCreateCampaign_Success(t *testing.T) {
	clientID := uuid.New()
	svc := &mocks.MockCampaignServicer{
		CreateCampaignFunc: func(_ context.Context, cid uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time) (*domain.Campaign, error) {
			c := newCampaign(cid)
			c.Name = name
			return c, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.CreateCampaign(context.Background(), &campaignv1.CreateCampaignRequest{
		ClientId:      clientID.String(),
		Name:          "My Campaign",
		ContactListId: uuid.New().String(),
	})

	require.NoError(t, err)
	assert.Equal(t, "My Campaign", resp.GetName())
	assert.Equal(t, clientID.String(), resp.GetClientId())
}

func TestCreateCampaign_MissingClientID(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.CreateCampaign(context.Background(), &campaignv1.CreateCampaignRequest{
		Name:          "My Campaign",
		ContactListId: uuid.New().String(),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCreateCampaign_InvalidClientIDFormat(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.CreateCampaign(context.Background(), &campaignv1.CreateCampaignRequest{
		ClientId:      "not-a-uuid",
		Name:          "My Campaign",
		ContactListId: uuid.New().String(),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCreateCampaign_MissingName(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.CreateCampaign(context.Background(), &campaignv1.CreateCampaignRequest{
		ClientId:      uuid.New().String(),
		ContactListId: uuid.New().String(),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "name is required")
}

func TestCreateCampaign_MissingContactListID(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.CreateCampaign(context.Background(), &campaignv1.CreateCampaignRequest{
		ClientId: uuid.New().String(),
		Name:     "Campaign",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "contact_list_id is required")
}

// ---- GetCampaign ----

func TestGetCampaign_Success(t *testing.T) {
	clientID := uuid.New()
	campaignID := uuid.New()
	expected := newCampaign(clientID)
	expected.ID = campaignID

	svc := &mocks.MockCampaignServicer{
		GetCampaignFunc: func(_ context.Context, id, cid uuid.UUID) (*domain.Campaign, error) {
			assert.Equal(t, campaignID, id)
			assert.Equal(t, clientID, cid)
			return expected, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.GetCampaign(context.Background(), &campaignv1.GetCampaignRequest{
		Id:       campaignID.String(),
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, campaignID.String(), resp.GetId())
}

func TestGetCampaign_NotFound(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		GetCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrCampaignNotFound
		},
	}

	srv := newServer(svc)
	_, err := srv.GetCampaign(context.Background(), &campaignv1.GetCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGetCampaign_MissingID(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.GetCampaign(context.Background(), &campaignv1.GetCampaignRequest{
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ---- ListCampaigns ----

func TestListCampaigns_Success(t *testing.T) {
	clientID := uuid.New()
	campaigns := []*domain.Campaign{
		newCampaign(clientID),
		newCampaign(clientID),
	}

	svc := &mocks.MockCampaignServicer{
		ListCampaignsFunc: func(_ context.Context, cid uuid.UUID, s string, limit, offset int) ([]*domain.Campaign, int, error) {
			return campaigns, 2, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.ListCampaigns(context.Background(), &campaignv1.ListCampaignsRequest{
		ClientId: clientID.String(),
		Limit:    10,
	})

	require.NoError(t, err)
	assert.Len(t, resp.GetCampaigns(), 2)
	assert.Equal(t, int32(2), resp.GetTotal())
}

func TestListCampaigns_MissingClientID(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.ListCampaigns(context.Background(), &campaignv1.ListCampaignsRequest{})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ---- UpdateCampaign ----

func TestUpdateCampaign_Success(t *testing.T) {
	clientID := uuid.New()
	campaignID := uuid.New()
	updated := newCampaign(clientID)
	updated.ID = campaignID
	updated.Name = "Updated Name"

	svc := &mocks.MockCampaignServicer{
		UpdateCampaignFunc: func(_ context.Context, id, cid uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time) (*domain.Campaign, error) {
			return updated, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.UpdateCampaign(context.Background(), &campaignv1.UpdateCampaignRequest{
		Id:       campaignID.String(),
		ClientId: clientID.String(),
		Name:     "Updated Name",
	})

	require.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.GetName())
}

func TestUpdateCampaign_NotDraft(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		UpdateCampaignFunc: func(_ context.Context, _, _ uuid.UUID, _, _, _, _, _ string, _ []string, _ int32, _ *time.Time) (*domain.Campaign, error) {
			return nil, domain.ErrCampaignNotDraft
		},
	}

	srv := newServer(svc)
	_, err := srv.UpdateCampaign(context.Background(), &campaignv1.UpdateCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ---- DeleteCampaign ----

func TestDeleteCampaign_Success(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		DeleteCampaignFunc: func(_ context.Context, _, _ uuid.UUID) error {
			return nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.DeleteCampaign(context.Background(), &campaignv1.DeleteCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestDeleteCampaign_NotFound(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		DeleteCampaignFunc: func(_ context.Context, _, _ uuid.UUID) error {
			return domain.ErrCampaignNotFound
		},
	}

	srv := newServer(svc)
	_, err := srv.DeleteCampaign(context.Background(), &campaignv1.DeleteCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ---- LaunchCampaign ----

func TestLaunchCampaign_Success(t *testing.T) {
	clientID := uuid.New()
	campaignID := uuid.New()
	launched := newCampaign(clientID)
	launched.ID = campaignID
	launched.Status = domain.StatusMaterializing

	svc := &mocks.MockCampaignServicer{
		LaunchCampaignFunc: func(_ context.Context, id, cid uuid.UUID) (*domain.Campaign, error) {
			assert.Equal(t, campaignID, id)
			assert.Equal(t, clientID, cid)
			return launched, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.LaunchCampaign(context.Background(), &campaignv1.LaunchCampaignRequest{
		Id:       campaignID.String(),
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusMaterializing, resp.GetStatus())
}

func TestLaunchCampaign_NotDraft(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		LaunchCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrCampaignNotDraft
		},
	}

	srv := newServer(svc)
	_, err := srv.LaunchCampaign(context.Background(), &campaignv1.LaunchCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestLaunchCampaign_NotFound(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		LaunchCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrCampaignNotFound
		},
	}

	srv := newServer(svc)
	_, err := srv.LaunchCampaign(context.Background(), &campaignv1.LaunchCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ---- PauseCampaign ----

func TestPauseCampaign_Success(t *testing.T) {
	clientID := uuid.New()
	c := newCampaign(clientID)
	c.Status = domain.StatusPaused

	svc := &mocks.MockCampaignServicer{
		PauseCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return c, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.PauseCampaign(context.Background(), &campaignv1.PauseCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusPaused, resp.GetStatus())
}

func TestPauseCampaign_NotRunning(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		PauseCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrCampaignNotRunning
		},
	}

	srv := newServer(svc)
	_, err := srv.PauseCampaign(context.Background(), &campaignv1.PauseCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ---- ResumeCampaign ----

func TestResumeCampaign_Success(t *testing.T) {
	clientID := uuid.New()
	c := newCampaign(clientID)
	c.Status = domain.StatusRunning

	svc := &mocks.MockCampaignServicer{
		ResumeCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return c, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.ResumeCampaign(context.Background(), &campaignv1.ResumeCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusRunning, resp.GetStatus())
}

func TestResumeCampaign_NotPaused(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		ResumeCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrCampaignNotPaused
		},
	}

	srv := newServer(svc)
	_, err := srv.ResumeCampaign(context.Background(), &campaignv1.ResumeCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ---- CancelCampaign ----

func TestCancelCampaign_Success(t *testing.T) {
	clientID := uuid.New()
	c := newCampaign(clientID)
	c.Status = domain.StatusCancelled

	svc := &mocks.MockCampaignServicer{
		CancelCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return c, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.CancelCampaign(context.Background(), &campaignv1.CancelCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, resp.GetStatus())
}

func TestCancelCampaign_InvalidStatus(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		CancelCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrInvalidCampaignStatus
		},
	}

	srv := newServer(svc)
	_, err := srv.CancelCampaign(context.Background(), &campaignv1.CancelCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ---- SetVariants ----

func TestSetVariants_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	variants := []domain.Variant{
		{ID: uuid.New(), CampaignID: campaignID, Name: "A", Percentage: 50},
		{ID: uuid.New(), CampaignID: campaignID, Name: "B", Percentage: 50},
	}

	svc := &mocks.MockCampaignServicer{
		SetVariantsFunc: func(_ context.Context, cid, clid uuid.UUID, v []domain.Variant) ([]domain.Variant, error) {
			return variants, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.SetVariants(context.Background(), &campaignv1.SetVariantsRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
		Variants: []*campaignv1.VariantInput{
			{Name: "A", Percentage: 50},
			{Name: "B", Percentage: 50},
		},
	})

	require.NoError(t, err)
	assert.Len(t, resp.GetVariants(), 2)
}

func TestSetVariants_TooFew(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		SetVariantsFunc: func(_ context.Context, _, _ uuid.UUID, _ []domain.Variant) ([]domain.Variant, error) {
			return nil, domain.ErrTooFewVariants
		},
	}

	srv := newServer(svc)
	_, err := srv.SetVariants(context.Background(), &campaignv1.SetVariantsRequest{
		CampaignId: uuid.New().String(),
		ClientId:   uuid.New().String(),
		Variants: []*campaignv1.VariantInput{
			{Name: "A", Percentage: 100},
		},
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestSetVariants_PercentageSumError(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		SetVariantsFunc: func(_ context.Context, _, _ uuid.UUID, _ []domain.Variant) ([]domain.Variant, error) {
			return nil, domain.ErrVariantPercentageSum
		},
	}

	srv := newServer(svc)
	_, err := srv.SetVariants(context.Background(), &campaignv1.SetVariantsRequest{
		CampaignId: uuid.New().String(),
		ClientId:   uuid.New().String(),
		Variants: []*campaignv1.VariantInput{
			{Name: "A", Percentage: 60},
			{Name: "B", Percentage: 60},
		},
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ---- SetABConfig ----

func TestSetABConfig_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	cfg := &domain.ABConfig{
		CampaignID:        campaignID,
		Metric:            "delivery_rate",
		TestDurationHours: 24,
		AutoSelectWinner:  true,
	}

	svc := &mocks.MockCampaignServicer{
		SetABConfigFunc: func(_ context.Context, cid, clid uuid.UUID, metric string, hours int32, auto bool) (*domain.ABConfig, error) {
			return cfg, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.SetABConfig(context.Background(), &campaignv1.SetABConfigRequest{
		CampaignId:        campaignID.String(),
		ClientId:          clientID.String(),
		Metric:            "delivery_rate",
		TestDurationHours: 24,
		AutoSelectWinner:  true,
	})

	require.NoError(t, err)
	assert.Equal(t, "delivery_rate", resp.GetMetric())
}

func TestSetABConfig_MissingMetric(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.SetABConfig(context.Background(), &campaignv1.SetABConfigRequest{
		CampaignId: uuid.New().String(),
		ClientId:   uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "metric is required")
}

// ---- SelectWinner ----

func TestSelectWinner_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	variantID := uuid.New()
	c := newCampaign(clientID)
	c.ID = campaignID

	svc := &mocks.MockCampaignServicer{
		SelectWinnerFunc: func(_ context.Context, cid, clid, vid uuid.UUID) (*domain.Campaign, error) {
			return c, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.SelectWinner(context.Background(), &campaignv1.SelectWinnerRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
		VariantId:  variantID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, campaignID.String(), resp.GetId())
}

func TestSelectWinner_AlreadySelected(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		SelectWinnerFunc: func(_ context.Context, _, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrWinnerAlreadySelected
		},
	}

	srv := newServer(svc)
	_, err := srv.SelectWinner(context.Background(), &campaignv1.SelectWinnerRequest{
		CampaignId: uuid.New().String(),
		ClientId:   uuid.New().String(),
		VariantId:  uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ---- SetRetryConfig ----

func TestSetRetryConfig_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	rc := &domain.RetryConfig{Enabled: true, MaxRetries: 3, DelayHours: 2}

	svc := &mocks.MockCampaignServicer{
		SetRetryConfigFunc: func(_ context.Context, _, _ uuid.UUID, config *domain.RetryConfig) (*domain.RetryConfig, error) {
			return config, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.SetRetryConfig(context.Background(), &campaignv1.SetRetryConfigRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
		Config: &campaignv1.RetryConfig{
			Enabled:    rc.Enabled,
			MaxRetries: rc.MaxRetries,
			DelayHours: rc.DelayHours,
		},
	})

	require.NoError(t, err)
	assert.True(t, resp.GetEnabled())
	assert.Equal(t, int32(3), resp.GetMaxRetries())
}

func TestSetRetryConfig_MissingConfig(t *testing.T) {
	srv := newServer(&mocks.MockCampaignServicer{})
	_, err := srv.SetRetryConfig(context.Background(), &campaignv1.SetRetryConfigRequest{
		CampaignId: uuid.New().String(),
		ClientId:   uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "config is required")
}

// ---- RetryFailed ----

func TestRetryFailed_Success(t *testing.T) {
	clientID := uuid.New()
	c := newCampaign(clientID)

	svc := &mocks.MockCampaignServicer{
		RetryFailedFunc: func(_ context.Context, _, _ uuid.UUID, altTemplate string) (*domain.Campaign, error) {
			return c, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.RetryFailed(context.Background(), &campaignv1.RetryFailedRequest{
		CampaignId: uuid.New().String(),
		ClientId:   clientID.String(),
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestRetryFailed_NoFailedRecipients(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		RetryFailedFunc: func(_ context.Context, _, _ uuid.UUID, _ string) (*domain.Campaign, error) {
			return nil, domain.ErrNoFailedRecipients
		},
	}

	srv := newServer(svc)
	_, err := srv.RetryFailed(context.Background(), &campaignv1.RetryFailedRequest{
		CampaignId: uuid.New().String(),
		ClientId:   uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ---- GetCampaignStats ----

func TestGetCampaignStats_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	snap := &domain.StatsSnapshot{
		CampaignID: campaignID,
		Sent:       100,
		Delivered:  90,
		Failed:     10,
	}
	counts := map[string]int32{
		domain.RecipientPending:   5,
		domain.RecipientSent:      100,
		domain.RecipientDelivered: 90,
		domain.RecipientFailed:    10,
	}

	svc := &mocks.MockCampaignServicer{
		GetStatsFunc: func(_ context.Context, cid, clid uuid.UUID) (*domain.StatsSnapshot, map[string]int32, []domain.Variant, error) {
			return snap, counts, nil, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.GetCampaignStats(context.Background(), &campaignv1.GetCampaignStatsRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, int32(100), resp.GetSent())
	assert.Equal(t, int32(90), resp.GetDelivered())
	assert.Equal(t, int32(10), resp.GetFailed())
	// delivery rate = 90/100 = 0.9
	assert.InDelta(t, 0.9, resp.GetDeliveryRate(), 0.001)
}

// ---- GetCampaignTimeline ----

func TestGetCampaignTimeline_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	now := time.Now()
	points := []domain.TimelinePoint{
		{Timestamp: now, Value: 10},
		{Timestamp: now.Add(time.Hour), Value: 20},
	}

	svc := &mocks.MockCampaignServicer{
		GetTimelineFunc: func(_ context.Context, _, _ uuid.UUID, interval, metric string) ([]domain.TimelinePoint, error) {
			return points, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.GetCampaignTimeline(context.Background(), &campaignv1.GetCampaignTimelineRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
		Interval:   "1h",
		Metric:     "sent",
	})

	require.NoError(t, err)
	assert.Len(t, resp.GetPoints(), 2)
	assert.Equal(t, int32(10), resp.GetPoints()[0].GetValue())
}

// ---- GetDeliveryHeatmap ----

func TestGetDeliveryHeatmap_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	cells := []domain.HeatmapCell{
		{DayOfWeek: 1, Hour: 10, DeliveredCount: 50, DeliveryRate: 0.9},
	}

	svc := &mocks.MockCampaignServicer{
		GetHeatmapFunc: func(_ context.Context, _, _ uuid.UUID) ([]domain.HeatmapCell, error) {
			return cells, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.GetDeliveryHeatmap(context.Background(), &campaignv1.GetDeliveryHeatmapRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
	})

	require.NoError(t, err)
	require.Len(t, resp.GetCells(), 1)
	assert.Equal(t, int32(1), resp.GetCells()[0].GetDayOfWeek())
	assert.InDelta(t, 0.9, resp.GetCells()[0].GetDeliveryRate(), 0.001)
}

// ---- GetVariantComparison ----

func TestGetVariantComparison_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()
	variantID := uuid.New()
	comparisons := []domain.VariantComparison{
		{VariantID: variantID, VariantName: "A", Sent: 100, Delivered: 90, DeliveryRate: 0.9},
	}

	svc := &mocks.MockCampaignServicer{
		GetVariantComparisonFunc: func(_ context.Context, _, _ uuid.UUID) ([]domain.VariantComparison, *uuid.UUID, error) {
			return comparisons, &variantID, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.GetVariantComparison(context.Background(), &campaignv1.GetVariantComparisonRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
	})

	require.NoError(t, err)
	require.Len(t, resp.GetRows(), 1)
	assert.Equal(t, "A", resp.GetRows()[0].GetVariantName())
	assert.Equal(t, variantID.String(), resp.GetWinnerVariantId())
}

// ---- GetOptimalSendTime ----

func TestGetOptimalSendTime_Success(t *testing.T) {
	clientID := uuid.New()
	slots := []domain.TimeSlot{
		{DayOfWeek: 2, Hour: 10, DeliveryRate: 0.95, Score: 9.5},
	}

	svc := &mocks.MockCampaignServicer{
		GetOptimalSendTimesFunc: func(_ context.Context, _ uuid.UUID) ([]domain.TimeSlot, error) {
			return slots, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.GetOptimalSendTime(context.Background(), &campaignv1.GetOptimalSendTimeRequest{
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	require.Len(t, resp.GetSlots(), 1)
	assert.Equal(t, int32(10), resp.GetSlots()[0].GetHour())
}

// ---- ExportReport ----

func TestExportReport_Success(t *testing.T) {
	campaignID := uuid.New()
	clientID := uuid.New()

	svc := &mocks.MockCampaignServicer{
		ExportReportFunc: func(_ context.Context, _, _ uuid.UUID, format string) ([]byte, string, string, error) {
			return []byte(`{"data":"ok"}`), "report.json", "application/json", nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.ExportReport(context.Background(), &campaignv1.ExportReportRequest{
		CampaignId: campaignID.String(),
		ClientId:   clientID.String(),
		Format:     "json",
	})

	require.NoError(t, err)
	assert.Equal(t, "report.json", resp.GetFilename())
	assert.Equal(t, "application/json", resp.GetContentType())
}

// ---- PreviewTemplate ----

func TestPreviewTemplate_Success(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		PreviewTemplateFunc: func(_ context.Context, input application.TemplatePreviewInput) ([]application.TemplatePreviewResult, error) {
			return []application.TemplatePreviewResult{
				{Rendered: "Hello, World!", Length: 13, Segments: 1},
			}, nil
		},
	}

	srv := newServer(svc)
	resp, err := srv.PreviewTemplate(context.Background(), &campaignv1.PreviewTemplateRequest{
		TemplateText: "Hello, World!",
		TestData: []*campaignv1.TemplateTestData{
			{Bindings: map[string]string{}},
		},
	})

	require.NoError(t, err)
	require.Len(t, resp.GetResults(), 1)
	assert.Equal(t, "Hello, World!", resp.GetResults()[0].GetRendered())
}

// ---- Error mapping ----

func TestMapError_VariantNotFound(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		GetCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, domain.ErrVariantNotFound
		},
	}

	srv := newServer(svc)
	_, err := srv.GetCampaign(context.Background(), &campaignv1.GetCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMapError_Internal(t *testing.T) {
	svc := &mocks.MockCampaignServicer{
		GetCampaignFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.Campaign, error) {
			return nil, assert.AnError
		},
	}

	srv := newServer(svc)
	_, err := srv.GetCampaign(context.Background(), &campaignv1.GetCampaignRequest{
		Id:       uuid.New().String(),
		ClientId: uuid.New().String(),
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}
