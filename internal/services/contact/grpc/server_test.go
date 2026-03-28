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

	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/mocks"
)

// helpers

func newServer(svc ContactServiceInterface) *Server {
	return &Server{service: svc}
}

func newContactList(clientID uuid.UUID) *domain.ContactList {
	return &domain.ContactList{
		ID:          uuid.New(),
		ClientID:    clientID,
		Name:        "Test List",
		Description: "desc",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func newContact(listID uuid.UUID) *domain.Contact {
	return &domain.Contact{
		ID:            uuid.New(),
		ContactListID: listID,
		Phone:         "+79001234567",
		Attributes:    map[string]interface{}{},
		Tags:          []string{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func newImportJob(listID, clientID uuid.UUID) *domain.ImportJob {
	return &domain.ImportJob{
		ID:            uuid.New(),
		ContactListID: listID,
		ClientID:      clientID,
		FileName:      "contacts.csv",
		FileSize:      1024,
		Status:        domain.ImportStatusPending,
		CreatedAt:     time.Now(),
	}
}

// --- CreateContactList ---

func TestCreateContactList_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	clientID := uuid.New()
	cl := newContactList(clientID)

	svc.On("CreateContactList", context.Background(), clientID, "My List", "desc").
		Return(cl, nil)

	resp, err := srv.CreateContactList(context.Background(), &contactv1.CreateContactListRequest{
		ClientId:    clientID.String(),
		Name:        "My List",
		Description: "desc",
	})

	require.NoError(t, err)
	assert.Equal(t, cl.ID.String(), resp.GetId())
	assert.Equal(t, cl.Name, resp.GetName())
	svc.AssertExpectations(t)
}

func TestCreateContactList_MissingClientID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.CreateContactList(context.Background(), &contactv1.CreateContactListRequest{
		ClientId: "",
		Name:     "My List",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	svc.AssertNotCalled(t, "CreateContactList")
}

func TestCreateContactList_MissingName(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.CreateContactList(context.Background(), &contactv1.CreateContactListRequest{
		ClientId: uuid.New().String(),
		Name:     "",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "name is required")
	svc.AssertNotCalled(t, "CreateContactList")
}

func TestCreateContactList_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	clientID := uuid.New()
	svc.On("CreateContactList", context.Background(), clientID, "X", "").
		Return(nil, domain.ErrContactListNotFound)

	_, err := srv.CreateContactList(context.Background(), &contactv1.CreateContactListRequest{
		ClientId: clientID.String(),
		Name:     "X",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// --- GetContactList ---

func TestGetContactList_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	clientID := uuid.New()
	cl := newContactList(clientID)

	svc.On("GetContactList", context.Background(), cl.ID, clientID).
		Return(cl, nil)

	resp, err := srv.GetContactList(context.Background(), &contactv1.GetContactListRequest{
		Id:       cl.ID.String(),
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, cl.ID.String(), resp.GetId())
	svc.AssertExpectations(t)
}

func TestGetContactList_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("GetContactList", context.Background(), listID, clientID).
		Return(nil, domain.ErrContactListNotFound)

	_, err := srv.GetContactList(context.Background(), &contactv1.GetContactListRequest{
		Id:       listID.String(),
		ClientId: clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	svc.AssertExpectations(t)
}

func TestGetContactList_InvalidID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.GetContactList(context.Background(), &contactv1.GetContactListRequest{
		Id:       "not-a-uuid",
		ClientId: uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- ListContactLists ---

func TestListContactLists_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	clientID := uuid.New()
	lists := []*domain.ContactList{
		newContactList(clientID),
		newContactList(clientID),
	}

	svc.On("ListContactLists", context.Background(), clientID, 10, 0).
		Return(lists, 2, nil)

	resp, err := srv.ListContactLists(context.Background(), &contactv1.ListContactListsRequest{
		ClientId: clientID.String(),
		Limit:    10,
		Offset:   0,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.GetTotal())
	assert.Len(t, resp.GetItems(), 2)
	svc.AssertExpectations(t)
}

func TestListContactLists_MissingClientID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.ListContactLists(context.Background(), &contactv1.ListContactListsRequest{
		ClientId: "",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- DeleteContactList ---

func TestDeleteContactList_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("DeleteContactList", context.Background(), listID, clientID).Return(nil)

	resp, err := srv.DeleteContactList(context.Background(), &contactv1.DeleteContactListRequest{
		Id:       listID.String(),
		ClientId: clientID.String(),
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	svc.AssertExpectations(t)
}

func TestDeleteContactList_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("DeleteContactList", context.Background(), listID, clientID).
		Return(domain.ErrContactListNotFound)

	_, err := srv.DeleteContactList(context.Background(), &contactv1.DeleteContactListRequest{
		Id:       listID.String(),
		ClientId: clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	svc.AssertExpectations(t)
}

// --- CreateContact ---

func TestCreateContact_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	c := newContact(listID)

	svc.On("CreateContact", context.Background(), listID, clientID, "+79001234567",
		map[string]interface{}{}, []string(nil)).
		Return(c, nil)

	resp, err := srv.CreateContact(context.Background(), &contactv1.CreateContactRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Phone:         "+79001234567",
	})

	require.NoError(t, err)
	assert.Equal(t, c.ID.String(), resp.GetId())
	assert.Equal(t, c.Phone, resp.GetPhone())
	svc.AssertExpectations(t)
}

func TestCreateContact_MissingPhone(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.CreateContact(context.Background(), &contactv1.CreateContactRequest{
		ContactListId: uuid.New().String(),
		ClientId:      uuid.New().String(),
		Phone:         "",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "phone is required")
}

func TestCreateContact_DuplicatePhone(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("CreateContact", context.Background(), listID, clientID, "+79001234567",
		map[string]interface{}{}, []string(nil)).
		Return(nil, domain.ErrDuplicatePhone)

	_, err := srv.CreateContact(context.Background(), &contactv1.CreateContactRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Phone:         "+79001234567",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

func TestCreateContact_InvalidPhone(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("CreateContact", context.Background(), listID, clientID, "bad_phone",
		map[string]interface{}{}, []string(nil)).
		Return(nil, domain.ErrInvalidPhone)

	_, err := srv.CreateContact(context.Background(), &contactv1.CreateContactRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Phone:         "bad_phone",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- DeleteContact ---

func TestDeleteContact_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	contactID := uuid.New()
	listID := uuid.New()
	clientID := uuid.New()

	svc.On("DeleteContact", context.Background(), contactID, listID, clientID).Return(nil)

	resp, err := srv.DeleteContact(context.Background(), &contactv1.DeleteContactRequest{
		Id:            contactID.String(),
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	svc.AssertExpectations(t)
}

func TestDeleteContact_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	contactID := uuid.New()
	listID := uuid.New()
	clientID := uuid.New()

	svc.On("DeleteContact", context.Background(), contactID, listID, clientID).
		Return(domain.ErrContactNotFound)

	_, err := srv.DeleteContact(context.Background(), &contactv1.DeleteContactRequest{
		Id:            contactID.String(),
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// --- PreviewSegment ---

func TestPreviewSegment_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("PreviewSegmentCount", context.Background(), listID, clientID,
		(*domain.SegmentRule)(nil), []string(nil)).
		Return(42, nil)

	resp, err := srv.PreviewSegment(context.Background(), &contactv1.PreviewSegmentRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, int32(42), resp.GetCount())
	svc.AssertExpectations(t)
}

func TestPreviewSegment_InvalidSegmentRules(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("PreviewSegmentCount", context.Background(), listID, clientID,
		(*domain.SegmentRule)(nil), []string(nil)).
		Return(0, domain.ErrInvalidSegmentRules)

	_, err := srv.PreviewSegment(context.Background(), &contactv1.PreviewSegmentRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestPreviewSegment_DepthExceeded(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("PreviewSegmentCount", context.Background(), listID, clientID,
		(*domain.SegmentRule)(nil), []string(nil)).
		Return(0, domain.ErrSegmentDepthExceeded)

	_, err := srv.PreviewSegment(context.Background(), &contactv1.PreviewSegmentRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- StartImport ---

func TestStartImport_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	importID := uuid.New()
	job := newImportJob(listID, clientID)
	job.ID = importID

	svc.On("StartImport", context.Background(), listID, clientID, importID, "data.csv", int64(2048), "").
		Return(job, nil)

	resp, err := srv.StartImport(context.Background(), &contactv1.StartImportRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		ImportId:      importID.String(),
		FileName:      "data.csv",
		FileSize:      2048,
	})

	require.NoError(t, err)
	assert.Equal(t, importID.String(), resp.GetId())
	assert.Equal(t, domain.ImportStatusPending, resp.GetStatus())
	svc.AssertExpectations(t)
}

func TestStartImport_MissingFileName(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.StartImport(context.Background(), &contactv1.StartImportRequest{
		ContactListId: uuid.New().String(),
		ClientId:      uuid.New().String(),
		ImportId:      uuid.New().String(),
		FileName:      "",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "file_name is required")
}

func TestStartImport_AlreadyStarted(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	importID := uuid.New()

	svc.On("StartImport", context.Background(), listID, clientID, importID, "data.csv", int64(0), "").
		Return(nil, domain.ErrImportAlreadyStarted)

	_, err := srv.StartImport(context.Background(), &contactv1.StartImportRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		ImportId:      importID.String(),
		FileName:      "data.csv",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// --- Error mapping ---

func TestMapError_AllCodes(t *testing.T) {
	srv := &Server{}

	cases := []struct {
		err      error
		wantCode codes.Code
	}{
		{domain.ErrContactListNotFound, codes.NotFound},
		{domain.ErrContactNotFound, codes.NotFound},
		{domain.ErrImportNotFound, codes.NotFound},
		{domain.ErrDuplicatePhone, codes.AlreadyExists},
		{domain.ErrInvalidPhone, codes.InvalidArgument},
		{domain.ErrInvalidSegmentRules, codes.InvalidArgument},
		{domain.ErrSegmentDepthExceeded, codes.InvalidArgument},
		{domain.ErrImportAlreadyStarted, codes.FailedPrecondition},
	}

	for _, tc := range cases {
		mapped := srv.mapError(tc.err)
		st, ok := status.FromError(mapped)
		require.True(t, ok)
		assert.Equal(t, tc.wantCode, st.Code(), "error: %v", tc.err)
	}
}

func TestMapError_UnknownError(t *testing.T) {
	srv := &Server{}
	mapped := srv.mapError(assert.AnError)
	st, _ := status.FromError(mapped)
	assert.Equal(t, codes.Internal, st.Code())
}

// --- UUID parsing helpers ---

func TestParseUUID_Empty(t *testing.T) {
	_, err := parseUUID("", "field")
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestParseUUID_Invalid(t *testing.T) {
	_, err := parseUUID("not-valid", "field")
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestParseUUID_Valid(t *testing.T) {
	id := uuid.New()
	parsed, err := parseUUID(id.String(), "field")
	require.NoError(t, err)
	assert.Equal(t, id, parsed)
}

func TestParseTwoUUIDs_Success(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	p1, p2, err := parseTwoUUIDs(id1.String(), "a", id2.String(), "b")
	require.NoError(t, err)
	assert.Equal(t, id1, p1)
	assert.Equal(t, id2, p2)
}

func TestParseTwoUUIDs_FirstInvalid(t *testing.T) {
	_, _, err := parseTwoUUIDs("bad", "a", uuid.New().String(), "b")
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestParseUUIDSlice_Success(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	result, err := parseUUIDSlice([]string{id1.String(), id2.String()}, "ids")
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{id1, id2}, result)
}

func TestParseUUIDSlice_Invalid(t *testing.T) {
	_, err := parseUUIDSlice([]string{"not-uuid"}, "ids")
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}
