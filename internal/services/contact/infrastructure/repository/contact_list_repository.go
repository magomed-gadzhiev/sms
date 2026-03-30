package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

// ContactListRepository handles CRUD operations for contact_lists.
type ContactListRepository struct {
	db *sqlx.DB
}

// NewContactListRepository creates a new ContactListRepository.
func NewContactListRepository(db *sqlx.DB) *ContactListRepository {
	return &ContactListRepository{db: db}
}

type contactListRow struct {
	ID            uuid.UUID    `db:"id"`
	ClientID      uuid.UUID    `db:"client_id"`
	Name          string       `db:"name"`
	Description   sql.NullString `db:"description"`
	ContactsCount int32        `db:"contacts_count"`
	CreatedAt     sql.NullTime `db:"created_at"`
	UpdatedAt     sql.NullTime `db:"updated_at"`
}

func (r *contactListRow) toDomain() *domain.ContactList {
	cl := &domain.ContactList{
		ID:            r.ID,
		ClientID:      r.ClientID,
		Name:          r.Name,
		ContactsCount: r.ContactsCount,
	}
	if r.Description.Valid {
		cl.Description = r.Description.String
	}
	if r.CreatedAt.Valid {
		cl.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		cl.UpdatedAt = r.UpdatedAt.Time
	}
	return cl
}

// Create inserts a new contact list.
func (r *ContactListRepository) Create(ctx context.Context, cl *domain.ContactList) (*domain.ContactList, error) {
	query := `INSERT INTO contact_lists (id, client_id, name, description, contacts_count)
		VALUES ($1, $2, $3, $4, 0)
		RETURNING id, client_id, name, description, contacts_count, created_at, updated_at`

	var row contactListRow
	err := r.db.QueryRowxContext(ctx, query, cl.ID, cl.ClientID, cl.Name, cl.Description).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("failed to create contact list: %w", err)
	}
	return row.toDomain(), nil
}

// GetByID retrieves a contact list by ID and client ID.
func (r *ContactListRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.ContactList, error) {
	query := `SELECT id, client_id, name, description, contacts_count, created_at, updated_at
		FROM contact_lists WHERE id = $1 AND client_id = $2`

	var row contactListRow
	err := r.db.QueryRowxContext(ctx, query, id, clientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactListNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contact list: %w", err)
	}
	return row.toDomain(), nil
}

// List retrieves contact lists for a client with pagination.
func (r *ContactListRepository) List(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.ContactList, int, error) {
	countQuery := `SELECT COUNT(*) FROM contact_lists WHERE client_id = $1`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, clientID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count contact lists: %w", err)
	}

	listQuery := `SELECT id, client_id, name, description, contacts_count, created_at, updated_at
		FROM contact_lists WHERE client_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, listQuery, clientID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list contact lists: %w", err)
	}
	defer rows.Close()

	var lists []*domain.ContactList
	for rows.Next() {
		var row contactListRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan contact list: %w", err)
		}
		lists = append(lists, row.toDomain())
	}
	return lists, total, nil
}

// Update updates a contact list's name and description.
func (r *ContactListRepository) Update(ctx context.Context, cl *domain.ContactList) (*domain.ContactList, error) {
	query := `UPDATE contact_lists SET name = $1, description = $2, updated_at = now()
		WHERE id = $3 AND client_id = $4
		RETURNING id, client_id, name, description, contacts_count, created_at, updated_at`

	var row contactListRow
	err := r.db.QueryRowxContext(ctx, query, cl.Name, cl.Description, cl.ID, cl.ClientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactListNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update contact list: %w", err)
	}
	return row.toDomain(), nil
}

// Delete removes a contact list.
func (r *ContactListRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	query := `DELETE FROM contact_lists WHERE id = $1 AND client_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete contact list: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrContactListNotFound
	}
	return nil
}

// UpdateContactsCount refreshes the contacts_count from actual contacts table.
func (r *ContactListRepository) UpdateContactsCount(ctx context.Context, contactListID uuid.UUID) error {
	query := `UPDATE contact_lists SET contacts_count = (
		SELECT COUNT(*) FROM contacts WHERE contact_list_id = $1
	), updated_at = now() WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, contactListID)
	if err != nil {
		return fmt.Errorf("failed to update contacts count: %w", err)
	}
	return nil
}

// Attribute row struct
type attributeRow struct {
	ID            uuid.UUID `db:"id"`
	ContactListID uuid.UUID `db:"contact_list_id"`
	Name          string    `db:"name"`
	DisplayName   string    `db:"display_name"`
	Type          string    `db:"type"`
	Required      bool      `db:"required"`
	Position      int32     `db:"position"`
}

func (r *attributeRow) toDomain() *domain.ContactAttribute {
	return &domain.ContactAttribute{
		ID:            r.ID,
		ContactListID: r.ContactListID,
		Name:          r.Name,
		DisplayName:   r.DisplayName,
		Type:          r.Type,
		Required:      r.Required,
		Position:      r.Position,
	}
}

// SetAttributes replaces all attributes for a contact list (delete + insert in a transaction).
func (r *ContactListRepository) SetAttributes(ctx context.Context, contactListID uuid.UUID, attrs []domain.ContactAttribute) ([]domain.ContactAttribute, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing attributes
	_, err = tx.ExecContext(ctx, `DELETE FROM contact_list_attributes WHERE contact_list_id = $1`, contactListID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old attributes: %w", err)
	}

	// Insert new attributes
	insertQuery := `INSERT INTO contact_list_attributes (id, contact_list_id, name, display_name, type, required, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, contact_list_id, name, display_name, type, required, position`

	var result []domain.ContactAttribute
	for _, attr := range attrs {
		if attr.ID == uuid.Nil {
			attr.ID = uuid.New()
		}
		var row attributeRow
		err := tx.QueryRowxContext(ctx, insertQuery,
			attr.ID, contactListID, attr.Name, attr.DisplayName, attr.Type, attr.Required, attr.Position,
		).StructScan(&row)
		if err != nil {
			return nil, fmt.Errorf("failed to insert attribute %s: %w", attr.Name, err)
		}
		result = append(result, *row.toDomain())
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit attributes: %w", err)
	}
	return result, nil
}

// GetAttributes retrieves all attributes for a contact list.
func (r *ContactListRepository) GetAttributes(ctx context.Context, contactListID uuid.UUID) ([]domain.ContactAttribute, error) {
	query := `SELECT id, contact_list_id, name, display_name, type, required, position
		FROM contact_list_attributes WHERE contact_list_id = $1
		ORDER BY position ASC`

	rows, err := r.db.QueryxContext(ctx, query, contactListID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attributes: %w", err)
	}
	defer rows.Close()

	var attrs []domain.ContactAttribute
	for rows.Next() {
		var row attributeRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan attribute: %w", err)
		}
		attrs = append(attrs, *row.toDomain())
	}
	return attrs, nil
}
