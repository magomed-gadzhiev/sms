package application

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
)

var phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)

// ContactService implements the business logic for the contact management domain.
type ContactService struct {
	listRepo    *repository.ContactListRepository
	contactRepo *repository.ContactRepository
	importRepo  *repository.ImportRepository
	logger      zerolog.Logger
}

// NewContactService creates a new ContactService.
func NewContactService(
	listRepo *repository.ContactListRepository,
	contactRepo *repository.ContactRepository,
	importRepo *repository.ImportRepository,
) *ContactService {
	return &ContactService{
		listRepo:    listRepo,
		contactRepo: contactRepo,
		importRepo:  importRepo,
		logger:      log.With().Str("component", "contact-service").Logger(),
	}
}

// --- Contact Lists ---

// CreateContactList creates a new contact list.
func (s *ContactService) CreateContactList(ctx context.Context, clientID uuid.UUID, name, description string) (*domain.ContactList, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	cl := &domain.ContactList{
		ID:          uuid.New(),
		ClientID:    clientID,
		Name:        name,
		Description: description,
	}

	return s.listRepo.Create(ctx, cl)
}

// GetContactList retrieves a contact list by ID, verifying ownership.
func (s *ContactService) GetContactList(ctx context.Context, id, clientID uuid.UUID) (*domain.ContactList, error) {
	return s.listRepo.GetByID(ctx, id, clientID)
}

// ListContactLists lists contact lists for a client.
func (s *ContactService) ListContactLists(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.ContactList, int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.listRepo.List(ctx, clientID, limit, offset)
}

// UpdateContactList updates a contact list's name and description.
func (s *ContactService) UpdateContactList(ctx context.Context, id, clientID uuid.UUID, name, description string) (*domain.ContactList, error) {
	existing, err := s.listRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if name != "" {
		existing.Name = name
	}
	existing.Description = description

	return s.listRepo.Update(ctx, existing)
}

// DeleteContactList deletes a contact list and cascades.
func (s *ContactService) DeleteContactList(ctx context.Context, id, clientID uuid.UUID) error {
	return s.listRepo.Delete(ctx, id, clientID)
}

// --- Attributes ---

// SetListAttributes replaces all attributes for a contact list.
func (s *ContactService) SetListAttributes(ctx context.Context, contactListID, clientID uuid.UUID, attrs []domain.ContactAttribute) ([]domain.ContactAttribute, error) {
	// Verify ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	return s.listRepo.SetAttributes(ctx, contactListID, attrs)
}

// GetListAttributes retrieves attributes for a contact list.
func (s *ContactService) GetListAttributes(ctx context.Context, contactListID, clientID uuid.UUID) ([]domain.ContactAttribute, error) {
	// Verify ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	return s.listRepo.GetAttributes(ctx, contactListID)
}

// --- Contacts ---

// CreateContact creates a new contact in a list.
func (s *ContactService) CreateContact(ctx context.Context, contactListID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	phone = normalizePhone(phone)
	if !phoneRegex.MatchString(phone) {
		return nil, domain.ErrInvalidPhone
	}

	if attrs == nil {
		attrs = make(map[string]interface{})
	}
	if tags == nil {
		tags = []string{}
	}

	c := &domain.Contact{
		ID:            uuid.New(),
		ContactListID: contactListID,
		Phone:         phone,
		Attributes:    attrs,
		Tags:          tags,
	}

	created, err := s.contactRepo.Create(ctx, c)
	if err != nil {
		return nil, err
	}

	// Update count asynchronously (best effort)
	go func() {
		if err := s.listRepo.UpdateContactsCount(context.Background(), contactListID); err != nil {
			s.logger.Error().Err(err).Str("list_id", contactListID.String()).Msg("failed to update contacts count")
		}
	}()

	return created, nil
}

// UpdateContact updates a contact.
func (s *ContactService) UpdateContact(ctx context.Context, id, contactListID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	existing, err := s.contactRepo.GetByID(ctx, id, contactListID)
	if err != nil {
		return nil, err
	}

	if phone != "" {
		phone = normalizePhone(phone)
		if !phoneRegex.MatchString(phone) {
			return nil, domain.ErrInvalidPhone
		}
		existing.Phone = phone
	}

	if attrs != nil {
		existing.Attributes = attrs
	}
	if tags != nil {
		existing.Tags = tags
	}

	return s.contactRepo.Update(ctx, existing)
}

// DeleteContact deletes a contact.
func (s *ContactService) DeleteContact(ctx context.Context, id, contactListID, clientID uuid.UUID) error {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return err
	}

	err := s.contactRepo.Delete(ctx, id, contactListID)
	if err != nil {
		return err
	}

	// Update count asynchronously
	go func() {
		if err := s.listRepo.UpdateContactsCount(context.Background(), contactListID); err != nil {
			s.logger.Error().Err(err).Str("list_id", contactListID.String()).Msg("failed to update contacts count")
		}
	}()

	return nil
}

// ListContacts lists contacts in a list.
func (s *ContactService) ListContacts(ctx context.Context, contactListID, clientID uuid.UUID, limit, offset int, search string, tags []string) ([]*domain.Contact, int, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.contactRepo.List(ctx, contactListID, limit, offset, search, tags)
}

// BatchUpsertContacts upserts contacts in batch.
func (s *ContactService) BatchUpsertContacts(ctx context.Context, contactListID, clientID uuid.UUID, contacts []domain.Contact) (*domain.BatchUpsertResult, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	result := &domain.BatchUpsertResult{}

	// Validate and normalize phones
	var validContacts []domain.Contact
	for i, c := range contacts {
		c.Phone = normalizePhone(c.Phone)
		if !phoneRegex.MatchString(c.Phone) {
			result.Errors++
			result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("row %d: invalid phone %s", i+1, c.Phone))
			continue
		}
		if c.Attributes == nil {
			c.Attributes = make(map[string]interface{})
		}
		if c.Tags == nil {
			c.Tags = []string{}
		}
		c.ContactListID = contactListID
		validContacts = append(validContacts, c)
	}

	if len(validContacts) > 0 {
		created, updated, err := s.contactRepo.BatchUpsert(ctx, contactListID, validContacts)
		if err != nil {
			return nil, err
		}
		result.Created = created
		result.Updated = updated
	}

	// Update count asynchronously
	go func() {
		if err := s.listRepo.UpdateContactsCount(context.Background(), contactListID); err != nil {
			s.logger.Error().Err(err).Str("list_id", contactListID.String()).Msg("failed to update contacts count")
		}
	}()

	return result, nil
}

// --- Tags ---

// AddTags adds tags to contacts.
func (s *ContactService) AddTags(ctx context.Context, contactListID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return err
	}

	return s.contactRepo.AddTags(ctx, contactListID, contactIDs, tags)
}

// RemoveTags removes tags from contacts.
func (s *ContactService) RemoveTags(ctx context.Context, contactListID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return err
	}

	return s.contactRepo.RemoveTags(ctx, contactListID, contactIDs, tags)
}

// ListTags returns distinct tags in a contact list.
func (s *ContactService) ListTags(ctx context.Context, contactListID, clientID uuid.UUID) ([]string, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	return s.contactRepo.ListTags(ctx, contactListID)
}

// --- Import ---

// StartImport creates an import job and launches async processing.
func (s *ContactService) StartImport(ctx context.Context, contactListID, clientID uuid.UUID, importID uuid.UUID, fileName string, fileSize int64, columnMappingJSON string) (*domain.ImportJob, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	// Parse column mapping
	var mapping []domain.ColumnMapping
	if columnMappingJSON != "" {
		if err := json.Unmarshal([]byte(columnMappingJSON), &mapping); err != nil {
			return nil, fmt.Errorf("invalid column_mapping JSON: %w", err)
		}
	}

	job := &domain.ImportJob{
		ID:            importID,
		ContactListID: contactListID,
		ClientID:      clientID,
		FileName:      fileName,
		FileSize:      fileSize,
		Status:        domain.ImportStatusPending,
		ColumnMapping: mapping,
	}

	created, err := s.importRepo.Create(ctx, job)
	if err != nil {
		return nil, err
	}

	// Launch async import worker
	worker := NewImportWorker(s.contactRepo, s.importRepo, s.listRepo, s.logger)
	go worker.Process(created)

	return created, nil
}

// GetImportStatus retrieves the status of an import job.
func (s *ContactService) GetImportStatus(ctx context.Context, id, contactListID, clientID uuid.UUID) (*domain.ImportJob, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, err
	}

	job, err := s.importRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify this import belongs to the given list and client
	if job.ContactListID != contactListID || job.ClientID != clientID {
		return nil, domain.ErrImportNotFound
	}

	return job, nil
}

// ListImports lists imports for a contact list.
func (s *ContactService) ListImports(ctx context.Context, contactListID, clientID uuid.UUID, limit, offset int) ([]*domain.ImportJob, int, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.importRepo.List(ctx, contactListID, clientID, limit, offset)
}

// --- Segmentation ---

// PreviewSegmentCount returns the count of contacts matching segment rules.
func (s *ContactService) PreviewSegmentCount(ctx context.Context, contactListID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string) (int32, error) {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return 0, err
	}

	where, params, err := domain.SegmentToSQL(rules, tags)
	if err != nil {
		return 0, err
	}

	return s.contactRepo.PreviewSegmentCount(ctx, contactListID, where, params)
}

// StreamSegment streams contacts matching segment rules via callback.
func (s *ContactService) StreamSegment(ctx context.Context, contactListID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string, fn func(*domain.Contact) error) error {
	// Verify list ownership
	if _, err := s.listRepo.GetByID(ctx, contactListID, clientID); err != nil {
		return err
	}

	where, params, err := domain.SegmentToSQL(rules, tags)
	if err != nil {
		return err
	}

	return s.contactRepo.StreamSegment(ctx, contactListID, where, params, fn)
}

// normalizePhone strips non-digit characters (except leading +) and ensures + prefix.
func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}

	hasPlus := strings.HasPrefix(phone, "+")

	// Strip non-digits
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}

	result := digits.String()
	if result == "" {
		return phone // return original if no digits found
	}

	if hasPlus || !strings.HasPrefix(result, "+") {
		result = "+" + result
	}

	return result
}
