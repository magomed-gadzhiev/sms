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

// --- UpdateContactList ---

func TestUpdateContactList_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	cl := newContactList(clientID)
	cl.ID = listID
	cl.Name = "Updated"

	svc.On("UpdateContactList", context.Background(), listID, clientID, "Updated", "new desc").
		Return(cl, nil)

	resp, err := srv.UpdateContactList(context.Background(), &contactv1.UpdateContactListRequest{
		Id:          listID.String(),
		ClientId:    clientID.String(),
		Name:        "Updated",
		Description: "new desc",
	})

	require.NoError(t, err)
	assert.Equal(t, "Updated", resp.GetName())
	svc.AssertExpectations(t)
}

func TestUpdateContactList_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("UpdateContactList", context.Background(), listID, clientID, "X", "").
		Return(nil, domain.ErrContactListNotFound)

	_, err := srv.UpdateContactList(context.Background(), &contactv1.UpdateContactListRequest{
		Id:       listID.String(),
		ClientId: clientID.String(),
		Name:     "X",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	svc.AssertExpectations(t)
}

func TestUpdateContactList_InvalidID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.UpdateContactList(context.Background(), &contactv1.UpdateContactListRequest{
		Id:       "bad-uuid",
		ClientId: uuid.New().String(),
		Name:     "X",
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- UpdateContact ---

func TestUpdateContact_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	contactID := uuid.New()
	listID := uuid.New()
	clientID := uuid.New()
	c := newContact(listID)
	c.ID = contactID
	c.Phone = "+79009999999"

	svc.On("UpdateContact", context.Background(), contactID, listID, clientID,
		"+79009999999", (map[string]interface{})(nil), []string(nil)).
		Return(c, nil)

	resp, err := srv.UpdateContact(context.Background(), &contactv1.UpdateContactRequest{
		Id:            contactID.String(),
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Phone:         "+79009999999",
	})

	require.NoError(t, err)
	assert.Equal(t, contactID.String(), resp.GetId())
	assert.Equal(t, "+79009999999", resp.GetPhone())
	svc.AssertExpectations(t)
}

func TestUpdateContact_InvalidContactID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.UpdateContact(context.Background(), &contactv1.UpdateContactRequest{
		Id:            "bad",
		ContactListId: uuid.New().String(),
		ClientId:      uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestUpdateContact_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	contactID := uuid.New()
	listID := uuid.New()
	clientID := uuid.New()

	svc.On("UpdateContact", context.Background(), contactID, listID, clientID,
		"", (map[string]interface{})(nil), []string(nil)).
		Return(nil, domain.ErrContactNotFound)

	_, err := srv.UpdateContact(context.Background(), &contactv1.UpdateContactRequest{
		Id:            contactID.String(),
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	svc.AssertExpectations(t)
}

// --- ListContacts ---

func TestListContacts_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	contacts := []*domain.Contact{
		newContact(listID),
		newContact(listID),
	}

	svc.On("ListContacts", context.Background(), listID, clientID, 20, 0, "", []string(nil)).
		Return(contacts, 2, nil)

	resp, err := srv.ListContacts(context.Background(), &contactv1.ListContactsRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Limit:         20,
		Offset:        0,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.GetTotal())
	assert.Len(t, resp.GetContacts(), 2)
	svc.AssertExpectations(t)
}

func TestListContacts_WithSearchAndTags(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("ListContacts", context.Background(), listID, clientID, 10, 5, "7900", []string{"vip"}).
		Return([]*domain.Contact{}, 0, nil)

	resp, err := srv.ListContacts(context.Background(), &contactv1.ListContactsRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Limit:         10,
		Offset:        5,
		Search:        "7900",
		Tags:          []string{"vip"},
	})

	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.GetTotal())
	svc.AssertExpectations(t)
}

func TestListContacts_InvalidListID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.ListContacts(context.Background(), &contactv1.ListContactsRequest{
		ContactListId: "bad",
		ClientId:      uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- BatchUpsertContacts ---

func TestBatchUpsertContacts_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	result := &domain.BatchUpsertResult{
		Created: 3,
		Updated: 1,
		Errors:  0,
	}

	svc.On("BatchUpsertContacts", context.Background(), listID, clientID,
		[]domain.Contact{
			{Phone: "+79001111111", Attributes: map[string]interface{}{}, Tags: []string(nil)},
			{Phone: "+79002222222", Attributes: map[string]interface{}{}, Tags: []string(nil)},
		}).
		Return(result, nil)

	resp, err := srv.BatchUpsertContacts(context.Background(), &contactv1.BatchUpsertContactsRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Contacts: []*contactv1.ContactInput{
			{Phone: "+79001111111"},
			{Phone: "+79002222222"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.GetCreated())
	assert.Equal(t, int32(1), resp.GetUpdated())
	svc.AssertExpectations(t)
}

func TestBatchUpsertContacts_InvalidListID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.BatchUpsertContacts(context.Background(), &contactv1.BatchUpsertContactsRequest{
		ContactListId: "bad",
		ClientId:      uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- AddTags ---

func TestAddTags_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	c1 := uuid.New()
	c2 := uuid.New()

	svc.On("AddTags", context.Background(), listID, clientID,
		[]uuid.UUID{c1, c2}, []string{"vip", "active"}).
		Return(nil)

	resp, err := srv.AddTags(context.Background(), &contactv1.AddTagsRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		ContactIds:    []string{c1.String(), c2.String()},
		Tags:          []string{"vip", "active"},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	svc.AssertExpectations(t)
}

func TestAddTags_EmptyTags(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.AddTags(context.Background(), &contactv1.AddTagsRequest{
		ContactListId: uuid.New().String(),
		ClientId:      uuid.New().String(),
		ContactIds:    []string{uuid.New().String()},
		Tags:          []string{},
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "tags are required")
}

func TestAddTags_InvalidContactID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.AddTags(context.Background(), &contactv1.AddTagsRequest{
		ContactListId: uuid.New().String(),
		ClientId:      uuid.New().String(),
		ContactIds:    []string{"not-uuid"},
		Tags:          []string{"vip"},
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- RemoveTags ---

func TestRemoveTags_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	c1 := uuid.New()

	svc.On("RemoveTags", context.Background(), listID, clientID,
		[]uuid.UUID{c1}, []string{"old"}).
		Return(nil)

	resp, err := srv.RemoveTags(context.Background(), &contactv1.RemoveTagsRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		ContactIds:    []string{c1.String()},
		Tags:          []string{"old"},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	svc.AssertExpectations(t)
}

func TestRemoveTags_EmptyTags(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.RemoveTags(context.Background(), &contactv1.RemoveTagsRequest{
		ContactListId: uuid.New().String(),
		ClientId:      uuid.New().String(),
		ContactIds:    []string{uuid.New().String()},
		Tags:          []string{},
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- ListTags ---

func TestListTags_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("ListTags", context.Background(), listID, clientID).
		Return([]string{"vip", "active", "new"}, nil)

	resp, err := srv.ListTags(context.Background(), &contactv1.ListTagsRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"vip", "active", "new"}, resp.GetTags())
	svc.AssertExpectations(t)
}

func TestListTags_InvalidListID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.ListTags(context.Background(), &contactv1.ListTagsRequest{
		ContactListId: "bad",
		ClientId:      uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- GetImportStatus ---

func TestGetImportStatus_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	importID := uuid.New()
	listID := uuid.New()
	clientID := uuid.New()
	job := newImportJob(listID, clientID)
	job.ID = importID
	job.Status = domain.ImportStatusCompleted
	job.ImportedCount = 500

	svc.On("GetImportStatus", context.Background(), importID, listID, clientID).
		Return(job, nil)

	resp, err := srv.GetImportStatus(context.Background(), &contactv1.GetImportStatusRequest{
		Id:            importID.String(),
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, importID.String(), resp.GetId())
	assert.Equal(t, domain.ImportStatusCompleted, resp.GetStatus())
	assert.Equal(t, int32(500), resp.GetImportedCount())
	svc.AssertExpectations(t)
}

func TestGetImportStatus_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	importID := uuid.New()
	listID := uuid.New()
	clientID := uuid.New()

	svc.On("GetImportStatus", context.Background(), importID, listID, clientID).
		Return(nil, domain.ErrImportNotFound)

	_, err := srv.GetImportStatus(context.Background(), &contactv1.GetImportStatusRequest{
		Id:            importID.String(),
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	svc.AssertExpectations(t)
}

func TestGetImportStatus_InvalidID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.GetImportStatus(context.Background(), &contactv1.GetImportStatusRequest{
		Id:            "bad",
		ContactListId: uuid.New().String(),
		ClientId:      uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- ListImports ---

func TestListImports_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	jobs := []*domain.ImportJob{
		newImportJob(listID, clientID),
		newImportJob(listID, clientID),
	}

	svc.On("ListImports", context.Background(), listID, clientID, 10, 0).
		Return(jobs, 2, nil)

	resp, err := srv.ListImports(context.Background(), &contactv1.ListImportsRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Limit:         10,
		Offset:        0,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.GetTotal())
	assert.Len(t, resp.GetImports(), 2)
	svc.AssertExpectations(t)
}

func TestListImports_InvalidListID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.ListImports(context.Background(), &contactv1.ListImportsRequest{
		ContactListId: "bad",
		ClientId:      uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- SetListAttributes ---

func TestSetListAttributes_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()
	attrID := uuid.New()

	resultAttrs := []domain.ContactAttribute{
		{
			ID:            attrID,
			ContactListID: listID,
			Name:          "first_name",
			DisplayName:   "First Name",
			Type:          domain.AttrTypeString,
			Required:      true,
			Position:      1,
		},
	}

	svc.On("SetListAttributes", context.Background(), listID, clientID,
		[]domain.ContactAttribute{
			{
				ContactListID: listID,
				Name:          "first_name",
				DisplayName:   "First Name",
				Type:          domain.AttrTypeString,
				Required:      true,
				Position:      1,
			},
		}).
		Return(resultAttrs, nil)

	resp, err := srv.SetListAttributes(context.Background(), &contactv1.SetListAttributesRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
		Attributes: []*contactv1.Attribute{
			{
				Name:        "first_name",
				DisplayName: "First Name",
				Type:        domain.AttrTypeString,
				Required:    true,
				Position:    1,
			},
		},
	})

	require.NoError(t, err)
	assert.Len(t, resp.GetAttributes(), 1)
	assert.Equal(t, "first_name", resp.GetAttributes()[0].GetName())
	assert.Equal(t, "First Name", resp.GetAttributes()[0].GetDisplayName())
	svc.AssertExpectations(t)
}

func TestSetListAttributes_InvalidListID(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	_, err := srv.SetListAttributes(context.Background(), &contactv1.SetListAttributesRequest{
		ContactListId: "bad",
		ClientId:      uuid.New().String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- GetListAttributes ---

func TestGetListAttributes_Success(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	attrs := []domain.ContactAttribute{
		{
			ID:            uuid.New(),
			ContactListID: listID,
			Name:          "email",
			DisplayName:   "Email",
			Type:          domain.AttrTypeString,
			Required:      false,
			Position:      1,
		},
		{
			ID:            uuid.New(),
			ContactListID: listID,
			Name:          "age",
			DisplayName:   "Age",
			Type:          domain.AttrTypeNumber,
			Required:      false,
			Position:      2,
		},
	}

	svc.On("GetListAttributes", context.Background(), listID, clientID).
		Return(attrs, nil)

	resp, err := srv.GetListAttributes(context.Background(), &contactv1.GetListAttributesRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	require.NoError(t, err)
	assert.Len(t, resp.GetAttributes(), 2)
	assert.Equal(t, "email", resp.GetAttributes()[0].GetName())
	assert.Equal(t, "age", resp.GetAttributes()[1].GetName())
	svc.AssertExpectations(t)
}

func TestGetListAttributes_NotFound(t *testing.T) {
	svc := &mocks.MockContactService{}
	srv := newServer(svc)

	listID := uuid.New()
	clientID := uuid.New()

	svc.On("GetListAttributes", context.Background(), listID, clientID).
		Return(nil, domain.ErrContactListNotFound)

	_, err := srv.GetListAttributes(context.Background(), &contactv1.GetListAttributesRequest{
		ContactListId: listID.String(),
		ClientId:      clientID.String(),
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	svc.AssertExpectations(t)
}
