//go:build functional

package functional_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/contact/application"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
)

func TestContactListChain(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-contact', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contacts WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_list_attributes WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_lists WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	listRepo := repository.NewContactListRepository(db)
	contactRepo := repository.NewContactRepository(db)
	importRepo := repository.NewImportRepository(db)
	svc := application.NewContactService(listRepo, contactRepo, importRepo)

	t.Run("CreateAndGetContactList", func(t *testing.T) {
		cl, err := svc.CreateContactList(ctx, clientID, "VIP Customers", "High-value segment")
		require.NoError(t, err)
		require.NotNil(t, cl)

		assert.Equal(t, "VIP Customers", cl.Name)
		assert.Equal(t, "High-value segment", cl.Description)
		assert.Equal(t, clientID, cl.ClientID)
		assert.Equal(t, int32(0), cl.ContactsCount)

		fetched, err := svc.GetContactList(ctx, cl.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, cl.ID, fetched.ID)
		assert.Equal(t, "VIP Customers", fetched.Name)
	})

	t.Run("CreateContactListEmptyNameFails", func(t *testing.T) {
		_, err := svc.CreateContactList(ctx, clientID, "", "description")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("ListContactLists", func(t *testing.T) {
		_, err := svc.CreateContactList(ctx, clientID, "List Alpha", "")
		require.NoError(t, err)
		_, err = svc.CreateContactList(ctx, clientID, "List Beta", "")
		require.NoError(t, err)

		lists, total, err := svc.ListContactLists(ctx, clientID, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(lists), 2)
	})

	t.Run("UpdateContactList", func(t *testing.T) {
		cl, err := svc.CreateContactList(ctx, clientID, "Original", "old desc")
		require.NoError(t, err)

		updated, err := svc.UpdateContactList(ctx, cl.ID, clientID, "Renamed", "new desc")
		require.NoError(t, err)
		assert.Equal(t, "Renamed", updated.Name)
		assert.Equal(t, "new desc", updated.Description)
	})

	t.Run("DeleteContactList", func(t *testing.T) {
		cl, err := svc.CreateContactList(ctx, clientID, "To Delete", "")
		require.NoError(t, err)

		err = svc.DeleteContactList(ctx, cl.ID, clientID)
		require.NoError(t, err)

		_, err = svc.GetContactList(ctx, cl.ID, clientID)
		assert.ErrorIs(t, err, domain.ErrContactListNotFound)
	})

	t.Run("ContactListNotFoundForOtherClient", func(t *testing.T) {
		cl, err := svc.CreateContactList(ctx, clientID, "Owned", "")
		require.NoError(t, err)

		otherClient := uuid.New()
		_, err = svc.GetContactList(ctx, cl.ID, otherClient)
		assert.ErrorIs(t, err, domain.ErrContactListNotFound)
	})
}

func TestContactCRUD(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-contact-crud', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contacts WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_list_attributes WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_lists WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	listRepo := repository.NewContactListRepository(db)
	contactRepo := repository.NewContactRepository(db)
	importRepo := repository.NewImportRepository(db)
	svc := application.NewContactService(listRepo, contactRepo, importRepo)

	// Create a contact list.
	cl, err := svc.CreateContactList(ctx, clientID, "Contact Test List", "")
	require.NoError(t, err)

	t.Run("CreateContact", func(t *testing.T) {
		attrs := map[string]interface{}{
			"name": "John Doe",
			"city": "Moscow",
		}
		tags := []string{"vip", "promo"}

		contact, err := svc.CreateContact(ctx, cl.ID, clientID, "+79001234567", attrs, tags)
		require.NoError(t, err)
		require.NotNil(t, contact)

		assert.Equal(t, "+79001234567", contact.Phone)
		assert.Equal(t, "John Doe", contact.Attributes["name"])
		assert.Equal(t, "Moscow", contact.Attributes["city"])
		assert.ElementsMatch(t, []string{"vip", "promo"}, contact.Tags)
	})

	t.Run("CreateContactInvalidPhoneFails", func(t *testing.T) {
		_, err := svc.CreateContact(ctx, cl.ID, clientID, "abc", nil, nil)
		assert.ErrorIs(t, err, domain.ErrInvalidPhone)
	})

	t.Run("CreateContactPhoneNormalization", func(t *testing.T) {
		// Phone without + prefix should be normalized.
		contact, err := svc.CreateContact(ctx, cl.ID, clientID, "79001234568", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "+79001234568", contact.Phone)
	})

	t.Run("UpdateContact", func(t *testing.T) {
		contact, err := svc.CreateContact(ctx, cl.ID, clientID, "+79001234569", nil, nil)
		require.NoError(t, err)

		newAttrs := map[string]interface{}{"name": "Jane Doe"}
		newTags := []string{"updated"}

		updated, err := svc.UpdateContact(ctx, contact.ID, cl.ID, clientID,
			"+79001234570", newAttrs, newTags)
		require.NoError(t, err)
		assert.Equal(t, "+79001234570", updated.Phone)
		assert.Equal(t, "Jane Doe", updated.Attributes["name"])
		assert.ElementsMatch(t, []string{"updated"}, updated.Tags)
	})

	t.Run("DeleteContact", func(t *testing.T) {
		contact, err := svc.CreateContact(ctx, cl.ID, clientID, "+79001234571", nil, nil)
		require.NoError(t, err)

		err = svc.DeleteContact(ctx, contact.ID, cl.ID, clientID)
		require.NoError(t, err)
	})

	t.Run("ListContacts", func(t *testing.T) {
		_, err := svc.CreateContact(ctx, cl.ID, clientID, "+79001234580", nil, nil)
		require.NoError(t, err)
		_, err = svc.CreateContact(ctx, cl.ID, clientID, "+79001234581", nil, nil)
		require.NoError(t, err)

		contacts, total, err := svc.ListContacts(ctx, cl.ID, clientID, 100, 0, "", nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(contacts), 2)
	})

	t.Run("BatchUpsertContacts", func(t *testing.T) {
		contacts := []domain.Contact{
			{Phone: "+79001234590", Attributes: map[string]interface{}{"name": "A"}},
			{Phone: "+79001234591", Attributes: map[string]interface{}{"name": "B"}},
			{Phone: "invalid"},
		}

		result, err := svc.BatchUpsertContacts(ctx, cl.ID, clientID, contacts)
		require.NoError(t, err)
		require.NotNil(t, result)

		// 2 valid contacts created, 1 error for invalid phone.
		assert.Equal(t, int32(2), result.Created)
		assert.Equal(t, int32(1), result.Errors)
		assert.Len(t, result.ErrorMessages, 1)
	})
}

func TestContactListAttributes(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-attrs', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contacts WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_list_attributes WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_lists WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	listRepo := repository.NewContactListRepository(db)
	contactRepo := repository.NewContactRepository(db)
	importRepo := repository.NewImportRepository(db)
	svc := application.NewContactService(listRepo, contactRepo, importRepo)

	cl, err := svc.CreateContactList(ctx, clientID, "Attrs List", "")
	require.NoError(t, err)

	t.Run("SetAndGetAttributes", func(t *testing.T) {
		attrs := []domain.ContactAttribute{
			{Name: "first_name", DisplayName: "First Name", Type: domain.AttrTypeString, Required: true, Position: 1},
			{Name: "age", DisplayName: "Age", Type: domain.AttrTypeNumber, Required: false, Position: 2},
		}

		result, err := svc.SetListAttributes(ctx, cl.ID, clientID, attrs)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "first_name", result[0].Name)
		assert.Equal(t, "First Name", result[0].DisplayName)
		assert.True(t, result[0].Required)

		fetched, err := svc.GetListAttributes(ctx, cl.ID, clientID)
		require.NoError(t, err)
		assert.Len(t, fetched, 2)
		assert.Equal(t, "first_name", fetched[0].Name)
		assert.Equal(t, "age", fetched[1].Name)
	})

	t.Run("ReplaceAttributes", func(t *testing.T) {
		// Replace with a different set.
		newAttrs := []domain.ContactAttribute{
			{Name: "email", DisplayName: "Email", Type: domain.AttrTypeString, Required: true, Position: 1},
		}

		result, err := svc.SetListAttributes(ctx, cl.ID, clientID, newAttrs)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "email", result[0].Name)

		// Old attributes should be gone.
		fetched, err := svc.GetListAttributes(ctx, cl.ID, clientID)
		require.NoError(t, err)
		assert.Len(t, fetched, 1)
	})
}

func TestContactTags(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-tags', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contacts WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_list_attributes WHERE contact_list_id IN (SELECT id FROM contact_lists WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM contact_lists WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	listRepo := repository.NewContactListRepository(db)
	contactRepo := repository.NewContactRepository(db)
	importRepo := repository.NewImportRepository(db)
	svc := application.NewContactService(listRepo, contactRepo, importRepo)

	cl, err := svc.CreateContactList(ctx, clientID, "Tags List", "")
	require.NoError(t, err)

	t.Run("AddAndRemoveTags", func(t *testing.T) {
		c1, err := svc.CreateContact(ctx, cl.ID, clientID, "+79001234600", nil, []string{"initial"})
		require.NoError(t, err)

		err = svc.AddTags(ctx, cl.ID, clientID, []uuid.UUID{c1.ID}, []string{"vip", "promo"})
		require.NoError(t, err)

		// Verify tags via list.
		tags, err := svc.ListTags(ctx, cl.ID, clientID)
		require.NoError(t, err)
		assert.Contains(t, tags, "vip")
		assert.Contains(t, tags, "promo")
		assert.Contains(t, tags, "initial")

		err = svc.RemoveTags(ctx, cl.ID, clientID, []uuid.UUID{c1.ID}, []string{"promo"})
		require.NoError(t, err)
	})
}
