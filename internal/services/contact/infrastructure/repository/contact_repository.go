package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

// ContactRepository handles CRUD for contacts, batch upsert, tags, and segmentation.
type ContactRepository struct {
	db *sqlx.DB
}

// NewContactRepository creates a new ContactRepository.
func NewContactRepository(db *sqlx.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

type contactRow struct {
	ID            uuid.UUID      `db:"id"`
	ContactListID uuid.UUID      `db:"contact_list_id"`
	Phone         string         `db:"phone"`
	Attributes    []byte         `db:"attributes"`
	Tags          pq.StringArray `db:"tags"`
	CreatedAt     sql.NullTime   `db:"created_at"`
	UpdatedAt     sql.NullTime   `db:"updated_at"`
}

func (r *contactRow) toDomain() (*domain.Contact, error) {
	c := &domain.Contact{
		ID:            r.ID,
		ContactListID: r.ContactListID,
		Phone:         r.Phone,
		Tags:          []string(r.Tags),
	}
	if r.Attributes != nil && len(r.Attributes) > 0 {
		attrs := make(map[string]interface{})
		if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
		}
		c.Attributes = attrs
	} else {
		c.Attributes = make(map[string]interface{})
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if r.CreatedAt.Valid {
		c.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		c.UpdatedAt = r.UpdatedAt.Time
	}
	return c, nil
}

// Create inserts a new contact.
func (r *ContactRepository) Create(ctx context.Context, c *domain.Contact) (*domain.Contact, error) {
	attrsJSON, err := json.Marshal(c.Attributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attributes: %w", err)
	}

	query := `INSERT INTO contacts (id, contact_list_id, phone, attributes, tags)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, contact_list_id, phone, attributes, tags, created_at, updated_at`

	var row contactRow
	err = r.db.QueryRowxContext(ctx, query,
		c.ID, c.ContactListID, c.Phone, string(attrsJSON), pq.StringArray(c.Tags),
	).StructScan(&row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicatePhone
		}
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}
	return row.toDomain()
}

// GetByID retrieves a single contact by ID within a contact list.
func (r *ContactRepository) GetByID(ctx context.Context, id, contactListID uuid.UUID) (*domain.Contact, error) {
	query := `SELECT id, contact_list_id, phone, attributes, tags, created_at, updated_at
		FROM contacts WHERE id = $1 AND contact_list_id = $2`

	var row contactRow
	err := r.db.QueryRowxContext(ctx, query, id, contactListID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}
	return row.toDomain()
}

// List retrieves contacts with pagination, optional search and tag filter.
func (r *ContactRepository) List(ctx context.Context, contactListID uuid.UUID, limit, offset int, search string, tags []string) ([]*domain.Contact, int, error) {
	args := []interface{}{contactListID}
	argIdx := 2

	where := `contact_list_id = $1`

	if search != "" {
		where += fmt.Sprintf(` AND phone ILIKE $%d`, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if len(tags) > 0 {
		where += fmt.Sprintf(` AND tags && $%d`, argIdx)
		args = append(args, pq.StringArray(tags))
		argIdx++
	}

	// Count
	countQuery := `SELECT COUNT(*) FROM contacts WHERE ` + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count contacts: %w", err)
	}

	// List
	listQuery := fmt.Sprintf(`SELECT id, contact_list_id, phone, attributes, tags, created_at, updated_at
		FROM contacts WHERE %s
		ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryxContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []*domain.Contact
	for rows.Next() {
		var row contactRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan contact: %w", err)
		}
		c, err := row.toDomain()
		if err != nil {
			return nil, 0, err
		}
		contacts = append(contacts, c)
	}
	return contacts, total, nil
}

// Update updates a contact's phone, attributes, and tags.
func (r *ContactRepository) Update(ctx context.Context, c *domain.Contact) (*domain.Contact, error) {
	attrsJSON, err := json.Marshal(c.Attributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attributes: %w", err)
	}

	query := `UPDATE contacts SET phone = $1, attributes = $2, tags = $3, updated_at = now()
		WHERE id = $4 AND contact_list_id = $5
		RETURNING id, contact_list_id, phone, attributes, tags, created_at, updated_at`

	var row contactRow
	err = r.db.QueryRowxContext(ctx, query,
		c.Phone, string(attrsJSON), pq.StringArray(c.Tags), c.ID, c.ContactListID,
	).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactNotFound
	}
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicatePhone
		}
		return nil, fmt.Errorf("failed to update contact: %w", err)
	}
	return row.toDomain()
}

// Delete removes a contact.
func (r *ContactRepository) Delete(ctx context.Context, id, contactListID uuid.UUID) error {
	query := `DELETE FROM contacts WHERE id = $1 AND contact_list_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, contactListID)
	if err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrContactNotFound
	}
	return nil
}

// BatchUpsert inserts or updates contacts in batch using ON CONFLICT.
// Returns created, updated counts. Uses xmax = 0 to detect insert vs update.
func (r *ContactRepository) BatchUpsert(ctx context.Context, contactListID uuid.UUID, contacts []domain.Contact) (int32, int32, error) {
	if len(contacts) == 0 {
		return 0, 0, nil
	}

	// Build multi-value INSERT
	var valueStrings []string
	var args []interface{}
	argIdx := 1

	for _, c := range contacts {
		attrsJSON, err := json.Marshal(c.Attributes)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to marshal attributes for %s: %w", c.Phone, err)
		}

		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d)",
			argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4,
		))

		id := c.ID
		if id == uuid.Nil {
			id = uuid.New()
		}

		args = append(args, id, contactListID, c.Phone, string(attrsJSON), pq.StringArray(c.Tags))
		argIdx += 5
	}

	query := fmt.Sprintf(`INSERT INTO contacts (id, contact_list_id, phone, attributes, tags)
		VALUES %s
		ON CONFLICT (contact_list_id, phone) DO UPDATE SET
			attributes = EXCLUDED.attributes,
			tags = EXCLUDED.tags,
			updated_at = now()
		RETURNING (xmax = 0) AS is_insert`, strings.Join(valueStrings, ", "))

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to batch upsert contacts: %w", err)
	}
	defer rows.Close()

	var created, updated int32
	for rows.Next() {
		var isInsert bool
		if err := rows.Scan(&isInsert); err != nil {
			return 0, 0, fmt.Errorf("failed to scan upsert result: %w", err)
		}
		if isInsert {
			created++
		} else {
			updated++
		}
	}
	return created, updated, nil
}

// AddTags adds tags to specified contacts using array_cat + array_distinct approach.
func (r *ContactRepository) AddTags(ctx context.Context, contactListID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	query := `UPDATE contacts
		SET tags = (SELECT ARRAY(SELECT DISTINCT unnest(tags || $1))),
			updated_at = now()
		WHERE contact_list_id = $2 AND id = ANY($3)`

	_, err := r.db.ExecContext(ctx, query, pq.StringArray(tags), contactListID, pq.Array(contactIDs))
	if err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}
	return nil
}

// RemoveTags removes tags from specified contacts.
func (r *ContactRepository) RemoveTags(ctx context.Context, contactListID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	query := `UPDATE contacts
		SET tags = (SELECT ARRAY(SELECT unnest(tags) EXCEPT SELECT unnest($1::text[]))),
			updated_at = now()
		WHERE contact_list_id = $2 AND id = ANY($3)`

	_, err := r.db.ExecContext(ctx, query, pq.StringArray(tags), contactListID, pq.Array(contactIDs))
	if err != nil {
		return fmt.Errorf("failed to remove tags: %w", err)
	}
	return nil
}

// ListTags returns all distinct tags used in a contact list.
func (r *ContactRepository) ListTags(ctx context.Context, contactListID uuid.UUID) ([]string, error) {
	query := `SELECT DISTINCT unnest(tags) AS tag FROM contacts WHERE contact_list_id = $1 ORDER BY tag`

	var tags []string
	rows, err := r.db.QueryContext(ctx, query, contactListID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// StreamSegment streams contacts matching segment rules via callback.
// The callback is called for each contact.
func (r *ContactRepository) StreamSegment(ctx context.Context, contactListID uuid.UUID, whereClause string, params []interface{}, fn func(*domain.Contact) error) error {
	query := `SELECT id, contact_list_id, phone, attributes, tags, created_at, updated_at
		FROM contacts WHERE contact_list_id = $1`

	allParams := []interface{}{contactListID}
	if whereClause != "" {
		query += " AND " + whereClause
		allParams = append(allParams, params...)
	}
	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryxContext(ctx, query, allParams...)
	if err != nil {
		return fmt.Errorf("failed to stream segment: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row contactRow
		if err := rows.StructScan(&row); err != nil {
			return fmt.Errorf("failed to scan contact in stream: %w", err)
		}
		c, err := row.toDomain()
		if err != nil {
			return err
		}
		if err := fn(c); err != nil {
			return err
		}
	}
	return rows.Err()
}

// PreviewSegmentCount returns the count of contacts matching segment rules.
func (r *ContactRepository) PreviewSegmentCount(ctx context.Context, contactListID uuid.UUID, whereClause string, params []interface{}) (int32, error) {
	query := `SELECT COUNT(*) FROM contacts WHERE contact_list_id = $1`

	allParams := []interface{}{contactListID}
	if whereClause != "" {
		query += " AND " + whereClause
		allParams = append(allParams, params...)
	}

	var count int32
	if err := r.db.QueryRowContext(ctx, query, allParams...).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to preview segment count: %w", err)
	}
	return count, nil
}
