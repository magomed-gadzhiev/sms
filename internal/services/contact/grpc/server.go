package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/services/contact/application"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

// ContactServiceInterface defines the application-layer operations required by the gRPC server.
type ContactServiceInterface interface {
	// Contact lists
	CreateContactList(ctx context.Context, clientID uuid.UUID, name, description string) (*domain.ContactList, error)
	GetContactList(ctx context.Context, id, clientID uuid.UUID) (*domain.ContactList, error)
	ListContactLists(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.ContactList, int, error)
	UpdateContactList(ctx context.Context, id, clientID uuid.UUID, name, description string) (*domain.ContactList, error)
	DeleteContactList(ctx context.Context, id, clientID uuid.UUID) error

	// Attributes
	SetListAttributes(ctx context.Context, contactListID, clientID uuid.UUID, attrs []domain.ContactAttribute) ([]domain.ContactAttribute, error)
	GetListAttributes(ctx context.Context, contactListID, clientID uuid.UUID) ([]domain.ContactAttribute, error)

	// Contacts
	CreateContact(ctx context.Context, contactListID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error)
	UpdateContact(ctx context.Context, id, contactListID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error)
	DeleteContact(ctx context.Context, id, contactListID, clientID uuid.UUID) error
	ListContacts(ctx context.Context, contactListID, clientID uuid.UUID, limit, offset int, search string, tags []string) ([]*domain.Contact, int, error)
	BatchUpsertContacts(ctx context.Context, contactListID, clientID uuid.UUID, contacts []domain.Contact) (*domain.BatchUpsertResult, error)

	// Tags
	AddTags(ctx context.Context, contactListID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error
	RemoveTags(ctx context.Context, contactListID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error
	ListTags(ctx context.Context, contactListID, clientID uuid.UUID) ([]string, error)

	// Import
	StartImport(ctx context.Context, contactListID, clientID uuid.UUID, importID uuid.UUID, fileName string, fileSize int64, columnMappingJSON string) (*domain.ImportJob, error)
	GetImportStatus(ctx context.Context, id, contactListID, clientID uuid.UUID) (*domain.ImportJob, error)
	ListImports(ctx context.Context, contactListID, clientID uuid.UUID, limit, offset int) ([]*domain.ImportJob, int, error)

	// Segmentation
	PreviewSegmentCount(ctx context.Context, contactListID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string) (int32, error)
	StreamSegment(ctx context.Context, contactListID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string, fn func(*domain.Contact) error) error
}

// Compile-time check that *application.ContactService satisfies the interface.
var _ ContactServiceInterface = (*application.ContactService)(nil)

// Server implements the gRPC ContactServiceServer.
type Server struct {
	contactv1.UnimplementedContactServiceServer
	service ContactServiceInterface
	logger  zerolog.Logger
}

// NewServer creates a new gRPC server for the contact service.
func NewServer(service ContactServiceInterface) *Server {
	return &Server{
		service: service,
		logger:  log.With().Str("component", "contact-grpc-server").Logger(),
	}
}

// --- Contact Lists ---

func (s *Server) CreateContactList(ctx context.Context, req *contactv1.CreateContactListRequest) (*contactv1.ContactList, error) {
	clientID, err := parseUUID(req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	cl, err := s.service.CreateContactList(ctx, clientID, req.GetName(), req.GetDescription())
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactListToProto(cl), nil
}

func (s *Server) ListContactLists(ctx context.Context, req *contactv1.ListContactListsRequest) (*contactv1.ContactListPage, error) {
	clientID, err := parseUUID(req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	lists, total, err := s.service.ListContactLists(ctx, clientID, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, s.mapError(err)
	}

	items := make([]*contactv1.ContactList, len(lists))
	for i, cl := range lists {
		items[i] = contactListToProto(cl)
	}

	return &contactv1.ContactListPage{
		Items: items,
		Total: int32(total),
	}, nil
}

func (s *Server) GetContactList(ctx context.Context, req *contactv1.GetContactListRequest) (*contactv1.ContactList, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	cl, err := s.service.GetContactList(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactListToProto(cl), nil
}

func (s *Server) UpdateContactList(ctx context.Context, req *contactv1.UpdateContactListRequest) (*contactv1.ContactList, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	cl, err := s.service.UpdateContactList(ctx, id, clientID, req.GetName(), req.GetDescription())
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactListToProto(cl), nil
}

func (s *Server) DeleteContactList(ctx context.Context, req *contactv1.DeleteContactListRequest) (*emptypb.Empty, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	if err := s.service.DeleteContactList(ctx, id, clientID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

// --- Attributes ---

func (s *Server) SetListAttributes(ctx context.Context, req *contactv1.SetListAttributesRequest) (*contactv1.AttributeList, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	attrs := protoAttrsToAttributes(req.GetAttributes(), contactListID)
	result, err := s.service.SetListAttributes(ctx, contactListID, clientID, attrs)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &contactv1.AttributeList{
		Attributes: attributeListToProto(result),
	}, nil
}

func (s *Server) GetListAttributes(ctx context.Context, req *contactv1.GetListAttributesRequest) (*contactv1.AttributeList, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	attrs, err := s.service.GetListAttributes(ctx, contactListID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &contactv1.AttributeList{
		Attributes: attributeListToProto(attrs),
	}, nil
}

// --- Contacts ---

func (s *Server) CreateContact(ctx context.Context, req *contactv1.CreateContactRequest) (*contactv1.Contact, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}
	if req.GetPhone() == "" {
		return nil, status.Error(codes.InvalidArgument, "phone is required")
	}

	attrs := structToMap(req.GetAttributes())
	c, err := s.service.CreateContact(ctx, contactListID, clientID, req.GetPhone(), attrs, req.GetTags())
	if err != nil {
		return nil, s.mapError(err)
	}

	proto, err := contactToProto(c)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return proto, nil
}

func (s *Server) UpdateContact(ctx context.Context, req *contactv1.UpdateContactRequest) (*contactv1.Contact, error) {
	id, err := parseUUID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	var attrs map[string]interface{}
	if req.GetAttributes() != nil {
		attrs = structToMap(req.GetAttributes())
	}

	c, err := s.service.UpdateContact(ctx, id, contactListID, clientID, req.GetPhone(), attrs, req.GetTags())
	if err != nil {
		return nil, s.mapError(err)
	}

	proto, err := contactToProto(c)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return proto, nil
}

func (s *Server) DeleteContact(ctx context.Context, req *contactv1.DeleteContactRequest) (*emptypb.Empty, error) {
	id, err := parseUUID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	if err := s.service.DeleteContact(ctx, id, contactListID, clientID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListContacts(ctx context.Context, req *contactv1.ListContactsRequest) (*contactv1.ContactPage, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	contacts, total, err := s.service.ListContacts(ctx, contactListID, clientID, int(req.GetLimit()), int(req.GetOffset()), req.GetSearch(), req.GetTags())
	if err != nil {
		return nil, s.mapError(err)
	}

	protoContacts := make([]*contactv1.Contact, 0, len(contacts))
	for _, c := range contacts {
		proto, err := contactToProto(c)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		protoContacts = append(protoContacts, proto)
	}

	return &contactv1.ContactPage{
		Contacts: protoContacts,
		Total:    int32(total),
	}, nil
}

func (s *Server) BatchUpsertContacts(ctx context.Context, req *contactv1.BatchUpsertContactsRequest) (*contactv1.BatchResult, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	// Convert proto contacts to domain
	contacts := make([]domain.Contact, 0, len(req.GetContacts()))
	for _, ci := range req.GetContacts() {
		contacts = append(contacts, domain.Contact{
			Phone:      ci.GetPhone(),
			Attributes: structToMap(ci.GetAttributes()),
			Tags:       ci.GetTags(),
		})
	}

	result, err := s.service.BatchUpsertContacts(ctx, contactListID, clientID, contacts)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &contactv1.BatchResult{
		Created:       result.Created,
		Updated:       result.Updated,
		Errors:        result.Errors,
		ErrorMessages: result.ErrorMessages,
	}, nil
}

// --- Tags ---

func (s *Server) AddTags(ctx context.Context, req *contactv1.AddTagsRequest) (*emptypb.Empty, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	contactIDs, err := parseUUIDSlice(req.GetContactIds(), "contact_ids")
	if err != nil {
		return nil, err
	}

	if len(req.GetTags()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "tags are required")
	}

	if err := s.service.AddTags(ctx, contactListID, clientID, contactIDs, req.GetTags()); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) RemoveTags(ctx context.Context, req *contactv1.RemoveTagsRequest) (*emptypb.Empty, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	contactIDs, err := parseUUIDSlice(req.GetContactIds(), "contact_ids")
	if err != nil {
		return nil, err
	}

	if len(req.GetTags()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "tags are required")
	}

	if err := s.service.RemoveTags(ctx, contactListID, clientID, contactIDs, req.GetTags()); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListTags(ctx context.Context, req *contactv1.ListTagsRequest) (*contactv1.TagList, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	tags, err := s.service.ListTags(ctx, contactListID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &contactv1.TagList{Tags: tags}, nil
}

// --- Import ---

func (s *Server) StartImport(ctx context.Context, req *contactv1.StartImportRequest) (*contactv1.ImportJob, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	importID, err := parseUUID(req.GetImportId(), "import_id")
	if err != nil {
		return nil, err
	}

	if req.GetFileName() == "" {
		return nil, status.Error(codes.InvalidArgument, "file_name is required")
	}

	job, err := s.service.StartImport(ctx, contactListID, clientID, importID, req.GetFileName(), req.GetFileSize(), req.GetColumnMapping())
	if err != nil {
		return nil, s.mapError(err)
	}

	return importJobToProto(job), nil
}

func (s *Server) GetImportStatus(ctx context.Context, req *contactv1.GetImportStatusRequest) (*contactv1.ImportJob, error) {
	id, err := parseUUID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	job, err := s.service.GetImportStatus(ctx, id, contactListID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	return importJobToProto(job), nil
}

func (s *Server) ListImports(ctx context.Context, req *contactv1.ListImportsRequest) (*contactv1.ImportJobPage, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	jobs, total, err := s.service.ListImports(ctx, contactListID, clientID, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, s.mapError(err)
	}

	protoJobs := make([]*contactv1.ImportJob, len(jobs))
	for i, job := range jobs {
		protoJobs[i] = importJobToProto(job)
	}

	return &contactv1.ImportJobPage{
		Imports: protoJobs,
		Total:   int32(total),
	}, nil
}

// --- Segmentation ---

func (s *Server) PreviewSegment(ctx context.Context, req *contactv1.PreviewSegmentRequest) (*contactv1.SegmentPreview, error) {
	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	rules := protoRuleToDomain(req.GetRules())

	count, err := s.service.PreviewSegmentCount(ctx, contactListID, clientID, rules, req.GetTags())
	if err != nil {
		return nil, s.mapError(err)
	}

	return &contactv1.SegmentPreview{Count: count}, nil
}

func (s *Server) StreamSegment(req *contactv1.StreamSegmentRequest, stream grpc.ServerStreamingServer[contactv1.ContactBatch]) error {
	ctx := stream.Context()

	contactListID, clientID, err := parseTwoUUIDs(req.GetContactListId(), "contact_list_id", req.GetClientId(), "client_id")
	if err != nil {
		return err
	}

	rules := protoRuleToDomain(req.GetRules())

	// Batch contacts for streaming (500 per batch)
	const batchSize = 500
	var batch []*contactv1.Contact

	err = s.service.StreamSegment(ctx, contactListID, clientID, rules, req.GetTags(), func(c *domain.Contact) error {
		proto, err := contactToProto(c)
		if err != nil {
			return err
		}
		batch = append(batch, proto)

		if len(batch) >= batchSize {
			if err := stream.Send(&contactv1.ContactBatch{Contacts: batch}); err != nil {
				return err
			}
			batch = batch[:0]
		}
		return nil
	})
	if err != nil {
		return s.mapError(err)
	}

	// Send remaining
	if len(batch) > 0 {
		if err := stream.Send(&contactv1.ContactBatch{Contacts: batch}); err != nil {
			return s.mapError(err)
		}
	}

	return nil
}

// --- Error mapping ---

func (s *Server) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrContactListNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrContactNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrImportNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDuplicatePhone):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidPhone):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidSegmentRules):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrSegmentDepthExceeded):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrImportAlreadyStarted):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		s.logger.Error().Err(err).Msg("internal error")
		return status.Error(codes.Internal, err.Error())
	}
}
