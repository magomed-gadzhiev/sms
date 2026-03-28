package grpc

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- Proto converters ---

func contactListToProto(cl *domain.ContactList) *contactv1.ContactList {
	return &contactv1.ContactList{
		Id:            cl.ID.String(),
		ClientId:      cl.ClientID.String(),
		Name:          cl.Name,
		Description:   cl.Description,
		ContactsCount: cl.ContactsCount,
		CreatedAt:     timestamppb.New(cl.CreatedAt),
		UpdatedAt:     timestamppb.New(cl.UpdatedAt),
	}
}

func contactToProto(c *domain.Contact) (*contactv1.Contact, error) {
	var attrs *structpb.Struct
	if c.Attributes != nil && len(c.Attributes) > 0 {
		var err error
		attrs, err = structpb.NewStruct(c.Attributes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert attributes to proto struct: %w", err)
		}
	}

	return &contactv1.Contact{
		Id:            c.ID.String(),
		ContactListId: c.ContactListID.String(),
		Phone:         c.Phone,
		Attributes:    attrs,
		Tags:          c.Tags,
		CreatedAt:     timestamppb.New(c.CreatedAt),
		UpdatedAt:     timestamppb.New(c.UpdatedAt),
	}, nil
}

func attributeToProto(attr *domain.ContactAttribute) *contactv1.Attribute {
	return &contactv1.Attribute{
		Id:          attr.ID.String(),
		Name:        attr.Name,
		DisplayName: attr.DisplayName,
		Type:        attr.Type,
		Required:    attr.Required,
		Position:    attr.Position,
	}
}

func attributeListToProto(attrs []domain.ContactAttribute) []*contactv1.Attribute {
	result := make([]*contactv1.Attribute, len(attrs))
	for i, attr := range attrs {
		result[i] = attributeToProto(&attr)
	}
	return result
}

func importJobToProto(job *domain.ImportJob) *contactv1.ImportJob {
	proto := &contactv1.ImportJob{
		Id:            job.ID.String(),
		ContactListId: job.ContactListID.String(),
		ClientId:      job.ClientID.String(),
		FileName:      job.FileName,
		FileSize:      job.FileSize,
		Status:        job.Status,
		TotalRows:     job.TotalRows,
		ImportedCount: job.ImportedCount,
		UpdatedCount:  job.UpdatedCount,
		ErrorCount:    job.ErrorCount,
		CreatedAt:     timestamppb.New(job.CreatedAt),
	}

	// Serialize errors to JSON string
	if job.Errors != nil {
		errorsJSON, err := json.Marshal(job.Errors)
		if err == nil {
			proto.Errors = string(errorsJSON)
		}
	}

	// Serialize column mapping to JSON string
	if job.ColumnMapping != nil {
		mappingJSON, err := json.Marshal(job.ColumnMapping)
		if err == nil {
			proto.ColumnMapping = string(mappingJSON)
		}
	}

	if job.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*job.CompletedAt)
	}

	return proto
}

// --- Proto to domain converters ---

func structToMap(s *structpb.Struct) map[string]interface{} {
	if s == nil {
		return make(map[string]interface{})
	}
	return s.AsMap()
}

func protoRuleToDomain(rule *contactv1.SegmentRule) *domain.SegmentRule {
	if rule == nil {
		return nil
	}

	domainRule := &domain.SegmentRule{
		Operator: rule.GetOperator(),
	}

	for _, cond := range rule.GetConditions() {
		domainRule.Conditions = append(domainRule.Conditions, domain.SegmentCondition{
			Field:  cond.GetField(),
			Op:     cond.GetOp(),
			Value:  cond.GetValue(),
			Value2: cond.GetValue2(),
		})
	}

	for _, nested := range rule.GetNested() {
		converted := protoRuleToDomain(nested)
		if converted != nil {
			domainRule.Nested = append(domainRule.Nested, *converted)
		}
	}

	return domainRule
}

func protoAttrsToAttributes(protoAttrs []*contactv1.Attribute, contactListID uuid.UUID) []domain.ContactAttribute {
	attrs := make([]domain.ContactAttribute, len(protoAttrs))
	for i, pa := range protoAttrs {
		var id uuid.UUID
		if pa.GetId() != "" {
			parsed, err := uuid.Parse(pa.GetId())
			if err == nil {
				id = parsed
			}
		}
		attrs[i] = domain.ContactAttribute{
			ID:            id,
			ContactListID: contactListID,
			Name:          pa.GetName(),
			DisplayName:   pa.GetDisplayName(),
			Type:          pa.GetType(),
			Required:      pa.GetRequired(),
			Position:      pa.GetPosition(),
		}
	}
	return attrs
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

func parseUUIDSlice(strs []string, fieldName string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid %s format: %s", fieldName, s)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
