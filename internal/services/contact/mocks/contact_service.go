package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/stretchr/testify/mock"
)

// MockContactService is a testify mock implementing grpc.ContactServiceInterface.
type MockContactService struct {
	mock.Mock
}

func (m *MockContactService) CreateContactList(ctx context.Context, clientID uuid.UUID, name, description string) (*domain.ContactList, error) {
	args := m.Called(ctx, clientID, name, description)
	if v := args.Get(0); v != nil {
		return v.(*domain.ContactList), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) GetContactList(ctx context.Context, id, clientID uuid.UUID) (*domain.ContactList, error) {
	args := m.Called(ctx, id, clientID)
	if v := args.Get(0); v != nil {
		return v.(*domain.ContactList), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) ListContactLists(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.ContactList, int, error) {
	args := m.Called(ctx, clientID, limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]*domain.ContactList), args.Int(1), args.Error(2)
	}
	return nil, args.Int(1), args.Error(2)
}

func (m *MockContactService) UpdateContactList(ctx context.Context, id, clientID uuid.UUID, name, description string) (*domain.ContactList, error) {
	args := m.Called(ctx, id, clientID, name, description)
	if v := args.Get(0); v != nil {
		return v.(*domain.ContactList), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) DeleteContactList(ctx context.Context, id, clientID uuid.UUID) error {
	args := m.Called(ctx, id, clientID)
	return args.Error(0)
}

func (m *MockContactService) SetListAttributes(ctx context.Context, contactListID, clientID uuid.UUID, attrs []domain.ContactAttribute) ([]domain.ContactAttribute, error) {
	args := m.Called(ctx, contactListID, clientID, attrs)
	if v := args.Get(0); v != nil {
		return v.([]domain.ContactAttribute), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) GetListAttributes(ctx context.Context, contactListID, clientID uuid.UUID) ([]domain.ContactAttribute, error) {
	args := m.Called(ctx, contactListID, clientID)
	if v := args.Get(0); v != nil {
		return v.([]domain.ContactAttribute), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) CreateContact(ctx context.Context, contactListID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error) {
	args := m.Called(ctx, contactListID, clientID, phone, attrs, tags)
	if v := args.Get(0); v != nil {
		return v.(*domain.Contact), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) UpdateContact(ctx context.Context, id, contactListID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error) {
	args := m.Called(ctx, id, contactListID, clientID, phone, attrs, tags)
	if v := args.Get(0); v != nil {
		return v.(*domain.Contact), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) DeleteContact(ctx context.Context, id, contactListID, clientID uuid.UUID) error {
	args := m.Called(ctx, id, contactListID, clientID)
	return args.Error(0)
}

func (m *MockContactService) ListContacts(ctx context.Context, contactListID, clientID uuid.UUID, limit, offset int, search string, tags []string) ([]*domain.Contact, int, error) {
	args := m.Called(ctx, contactListID, clientID, limit, offset, search, tags)
	if v := args.Get(0); v != nil {
		return v.([]*domain.Contact), args.Int(1), args.Error(2)
	}
	return nil, args.Int(1), args.Error(2)
}

func (m *MockContactService) BatchUpsertContacts(ctx context.Context, contactListID, clientID uuid.UUID, contacts []domain.Contact) (*domain.BatchUpsertResult, error) {
	args := m.Called(ctx, contactListID, clientID, contacts)
	if v := args.Get(0); v != nil {
		return v.(*domain.BatchUpsertResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) AddTags(ctx context.Context, contactListID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	args := m.Called(ctx, contactListID, clientID, contactIDs, tags)
	return args.Error(0)
}

func (m *MockContactService) RemoveTags(ctx context.Context, contactListID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	args := m.Called(ctx, contactListID, clientID, contactIDs, tags)
	return args.Error(0)
}

func (m *MockContactService) ListTags(ctx context.Context, contactListID, clientID uuid.UUID) ([]string, error) {
	args := m.Called(ctx, contactListID, clientID)
	if v := args.Get(0); v != nil {
		return v.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) StartImport(ctx context.Context, contactListID, clientID uuid.UUID, importID uuid.UUID, fileName string, fileSize int64, columnMappingJSON string) (*domain.ImportJob, error) {
	args := m.Called(ctx, contactListID, clientID, importID, fileName, fileSize, columnMappingJSON)
	if v := args.Get(0); v != nil {
		return v.(*domain.ImportJob), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) GetImportStatus(ctx context.Context, id, contactListID, clientID uuid.UUID) (*domain.ImportJob, error) {
	args := m.Called(ctx, id, contactListID, clientID)
	if v := args.Get(0); v != nil {
		return v.(*domain.ImportJob), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockContactService) ListImports(ctx context.Context, contactListID, clientID uuid.UUID, limit, offset int) ([]*domain.ImportJob, int, error) {
	args := m.Called(ctx, contactListID, clientID, limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]*domain.ImportJob), args.Int(1), args.Error(2)
	}
	return nil, args.Int(1), args.Error(2)
}

func (m *MockContactService) PreviewSegmentCount(ctx context.Context, contactListID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string) (int32, error) {
	args := m.Called(ctx, contactListID, clientID, rules, tags)
	return int32(args.Int(0)), args.Error(1)
}

func (m *MockContactService) StreamSegment(ctx context.Context, contactListID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string, fn func(*domain.Contact) error) error {
	args := m.Called(ctx, contactListID, clientID, rules, tags, fn)
	return args.Error(0)
}
