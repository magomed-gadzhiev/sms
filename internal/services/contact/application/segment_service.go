package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
)

type SegmentService struct {
	segmentRepo *repository.SegmentRepository
	pool        *pgxpool.Pool
	logger      zerolog.Logger
}

func NewSegmentService(segmentRepo *repository.SegmentRepository, pool *pgxpool.Pool) *SegmentService {
	return &SegmentService{
		segmentRepo: segmentRepo,
		pool:        pool,
		logger:      log.With().Str("component", "segment-service").Logger(),
	}
}

func (s *SegmentService) Create(ctx context.Context, seg *domain.SavedSegment) error {
	return s.segmentRepo.Create(ctx, seg)
}

func (s *SegmentService) Get(ctx context.Context, id, clientID uuid.UUID) (*domain.SavedSegment, error) {
	return s.segmentRepo.GetByID(ctx, id, clientID)
}

func (s *SegmentService) List(ctx context.Context, clientID uuid.UUID) ([]*domain.SavedSegment, error) {
	return s.segmentRepo.ListByClient(ctx, clientID)
}

func (s *SegmentService) Update(ctx context.Context, seg *domain.SavedSegment) error {
	return s.segmentRepo.Update(ctx, seg)
}

func (s *SegmentService) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	return s.segmentRepo.Delete(ctx, id, clientID)
}

// EstimateCount runs the segment query and returns the count of matching contacts.
func (s *SegmentService) EstimateCount(ctx context.Context, seg *domain.SavedSegment) (int32, error) {
	whereClause, args, err := BuildWhereClause(seg.Rules, 2) // $1 is contact_list_ids
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(
		`SELECT COUNT(DISTINCT phone) FROM contacts WHERE contact_list_id = ANY($1::uuid[]) AND (%s)`,
		whereClause,
	)
	allArgs := append([]interface{}{seg.ContactListIDs}, args...)

	var count int32
	err = s.pool.QueryRow(ctx, query, allArgs...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("estimate count: %w", err)
	}

	_ = s.segmentRepo.UpdateEstimate(ctx, seg.ID, count)
	return count, nil
}

// BuildWhereClause translates segment rules to a parameterized SQL WHERE clause.
// startParam is the next available $N parameter index.
func BuildWhereClause(rules domain.SegmentRules, startParam int) (string, []interface{}, error) {
	if len(rules.Conditions) == 0 {
		return "TRUE", nil, nil
	}
	return buildGroup(rules.Operator, rules.Conditions, startParam)
}

func buildGroup(operator string, conditions []domain.SegmentRuleNode, paramIdx int) (string, []interface{}, error) {
	if operator != "AND" && operator != "OR" {
		operator = "AND"
	}

	var parts []string
	var allArgs []interface{}

	for _, cond := range conditions {
		if cond.IsGroup() {
			clause, args, err := buildGroup(cond.Operator, cond.Conditions, paramIdx)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, "("+clause+")")
			allArgs = append(allArgs, args...)
			paramIdx += len(args)
		} else {
			clause, args, err := buildCondition(cond, paramIdx)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, clause)
			allArgs = append(allArgs, args...)
			paramIdx += len(args)
		}
	}

	return strings.Join(parts, " "+operator+" "), allArgs, nil
}

func buildCondition(cond domain.SegmentRuleNode, paramIdx int) (string, []interface{}, error) {
	field := cond.Field
	// Extract JSONB path: "attributes.city" -> "attributes->>'city'"
	attrName := strings.TrimPrefix(field, "attributes.")
	col := fmt.Sprintf("attributes->>'%s'", attrName)

	switch cond.Op {
	case "eq":
		return fmt.Sprintf("%s = $%d", col, paramIdx), []interface{}{fmt.Sprintf("%v", cond.Value)}, nil
	case "neq":
		return fmt.Sprintf("(%s IS NULL OR %s != $%d)", col, col, paramIdx), []interface{}{fmt.Sprintf("%v", cond.Value)}, nil
	case "gt":
		return fmt.Sprintf("(%s)::numeric > $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "gte":
		return fmt.Sprintf("(%s)::numeric >= $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "lt":
		return fmt.Sprintf("(%s)::numeric < $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "lte":
		return fmt.Sprintf("(%s)::numeric <= $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "contains":
		return fmt.Sprintf("%s ILIKE $%d", col, paramIdx), []interface{}{fmt.Sprintf("%%%v%%", cond.Value)}, nil
	case "not_contains":
		return fmt.Sprintf("(%s IS NULL OR %s NOT ILIKE $%d)", col, col, paramIdx), []interface{}{fmt.Sprintf("%%%v%%", cond.Value)}, nil
	case "in":
		values, ok := cond.Value.([]interface{})
		if !ok {
			return "", nil, fmt.Errorf("in operator requires array value")
		}
		placeholders := make([]string, len(values))
		args := make([]interface{}, len(values))
		for i, v := range values {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx+i)
			args[i] = fmt.Sprintf("%v", v)
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")), args, nil
	case "is_empty":
		return fmt.Sprintf("(%s IS NULL OR %s = '')", col, col), nil, nil
	case "is_not_empty":
		return fmt.Sprintf("(%s IS NOT NULL AND %s != '')", col, col), nil, nil
	case "older_than_days":
		days := fmt.Sprintf("%v", cond.Value)
		return fmt.Sprintf("(%s)::date < (CURRENT_DATE - INTERVAL '%s days')", col, days), nil, nil
	case "newer_than_days":
		days := fmt.Sprintf("%v", cond.Value)
		return fmt.Sprintf("(%s)::date > (CURRENT_DATE - INTERVAL '%s days')", col, days), nil, nil
	default:
		return "", nil, fmt.Errorf("unknown operator: %s", cond.Op)
	}
}
