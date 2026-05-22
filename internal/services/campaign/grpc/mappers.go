package grpc

import (
	"encoding/json"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

// --- Domain to Proto ---

func campaignToProto(c *domain.Campaign) *campaignv1.Campaign {
	pb := &campaignv1.Campaign{
		Id:              c.ID.String(),
		ClientId:        c.ClientID.String(),
		Name:            c.Name,
		Status:          c.Status,
		ContactListId:   c.ContactListID.String(),
		Source:          c.Source,
		SegmentRules:    c.SegmentRules,
		SegmentTags:     c.SegmentTags,
		SendRate:        c.SendRate,
		TotalRecipients:       c.TotalRecipients,
		SentCount:             c.SentCount,
		DeliveredCount:        c.DeliveredCount,
		FailedCount:           c.FailedCount,
		CreatedAt:             timestamppb.New(c.CreatedAt),
		UpdatedAt:             timestamppb.New(c.UpdatedAt),
		UseSubscriberTimezone: c.UseSubscriberTimezone,
	}

	if c.TemplateID != nil {
		pb.TemplateId = c.TemplateID.String()
	}
	if c.ScheduledAt != nil {
		pb.ScheduledAt = timestamppb.New(*c.ScheduledAt)
	}
	if c.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*c.StartedAt)
	}
	if c.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*c.CompletedAt)
	}
	if c.RetryConfig != nil {
		data, err := json.Marshal(c.RetryConfig)
		if err == nil {
			pb.RetryConfig = string(data)
		}
	}

	// Map variants
	if len(c.Variants) > 0 {
		pb.Variants = make([]*campaignv1.Variant, len(c.Variants))
		for i, v := range c.Variants {
			pb.Variants[i] = variantToProto(&v)
		}
	}

	// Map AB config
	if c.ABConfig != nil {
		pb.AbConfig = abConfigToProto(c.ABConfig)
	}

	return pb
}

func variantToProto(v *domain.Variant) *campaignv1.Variant {
	pb := &campaignv1.Variant{
		Id:             v.ID.String(),
		CampaignId:     v.CampaignID.String(),
		Name:           v.Name,
		Percentage:     v.Percentage,
		IsWinner:       v.IsWinner,
		IsControl:      v.IsControl,
		SentCount:      v.SentCount,
		DeliveredCount: v.DeliveredCount,
		FailedCount:    v.FailedCount,
	}
	if v.TemplateID != nil {
		pb.TemplateId = v.TemplateID.String()
	}
	return pb
}

func abConfigToProto(cfg *domain.ABConfig) *campaignv1.ABConfig {
	pb := &campaignv1.ABConfig{
		CampaignId:        cfg.CampaignID.String(),
		Metric:            cfg.Metric,
		TestDurationHours: cfg.TestDurationHours,
		AutoSelectWinner:  cfg.AutoSelectWinner,
	}
	if cfg.WinnerVariantID != nil {
		pb.WinnerVariantId = cfg.WinnerVariantID.String()
	}
	if cfg.WinnerSelectedAt != nil {
		pb.WinnerSelectedAt = timestamppb.New(*cfg.WinnerSelectedAt)
	}
	return pb
}

func retryConfigToProto(rc *domain.RetryConfig) *campaignv1.RetryConfig {
	return &campaignv1.RetryConfig{
		Enabled:               rc.Enabled,
		DelayHours:            rc.DelayHours,
		MaxRetries:            rc.MaxRetries,
		AlternativeTemplateId: rc.AlternativeTemplateID,
	}
}

func variantListToProto(variants []domain.Variant) *campaignv1.VariantList {
	pb := &campaignv1.VariantList{
		Variants: make([]*campaignv1.Variant, len(variants)),
	}
	for i, v := range variants {
		pb.Variants[i] = variantToProto(&v)
	}
	return pb
}

// --- Proto to Domain ---

func protoRetryConfigToDomain(pb *campaignv1.RetryConfig) *domain.RetryConfig {
	if pb == nil {
		return nil
	}
	return &domain.RetryConfig{
		Enabled:               pb.GetEnabled(),
		DelayHours:            pb.GetDelayHours(),
		MaxRetries:            pb.GetMaxRetries(),
		AlternativeTemplateID: pb.GetAlternativeTemplateId(),
	}
}

func protoVariantInputsToDomain(inputs []*campaignv1.VariantInput) []domain.Variant {
	variants := make([]domain.Variant, len(inputs))
	for i, input := range inputs {
		v := domain.Variant{
			Name:       input.GetName(),
			Percentage: input.GetPercentage(),
			IsControl:  input.GetIsControl(),
		}
		if input.GetTemplateId() != "" {
			id, err := uuid.Parse(input.GetTemplateId())
			if err == nil {
				v.TemplateID = &id
			}
		}
		variants[i] = v
	}
	return variants
}

// --- UUID parsing helpers ---

func parseUUID(s, fieldName string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s is required", fieldName)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s format", fieldName)
	}
	return id, nil
}

func parseTwoUUIDs(s1, name1, s2, name2 string) (uuid.UUID, uuid.UUID, error) {
	id1, err := parseUUID(s1, name1)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	id2, err := parseUUID(s2, name2)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return id1, id2, nil
}
