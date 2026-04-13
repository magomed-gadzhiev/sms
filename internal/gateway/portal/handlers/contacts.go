package handlers

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"

	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ContactHandlers содержит HTTP обработчики для контактов и списков контактов
type ContactHandlers struct {
	contactClient contactv1.ContactServiceClient
}

// NewContactHandlers создаёт новый экземпляр ContactHandlers
func NewContactHandlers(contactClient contactv1.ContactServiceClient) *ContactHandlers {
	return &ContactHandlers{contactClient: contactClient}
}

// CreateContactList обрабатывает POST /contact-lists
func (h *ContactHandlers) CreateContactList(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}

	resp, err := h.contactClient.CreateContactList(r.Context(), &contactv1.CreateContactListRequest{
		ClientId:    clientID.String(),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// ListContactLists обрабатывает GET /contact-lists
func (h *ContactHandlers) ListContactLists(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage

	resp, err := h.contactClient.ListContactLists(r.Context(), &contactv1.ListContactListsRequest{
		ClientId: clientID.String(),
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetContactList обрабатывает GET /contact-lists/{id}
func (h *ContactHandlers) GetContactList(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.contactClient.GetContactList(r.Context(), &contactv1.GetContactListRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// UpdateContactList обрабатывает PUT /contact-lists/{id}
func (h *ContactHandlers) UpdateContactList(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.contactClient.UpdateContactList(r.Context(), &contactv1.UpdateContactListRequest{
		Id:          id,
		ClientId:    clientID.String(),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// DeleteContactList обрабатывает DELETE /contact-lists/{id}
func (h *ContactHandlers) DeleteContactList(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	_, err := h.contactClient.DeleteContactList(r.Context(), &contactv1.DeleteContactListRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetListAttributes обрабатывает PUT /contact-lists/{id}/attributes
func (h *ContactHandlers) SetListAttributes(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		Attributes []*contactv1.Attribute `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.contactClient.SetListAttributes(r.Context(), &contactv1.SetListAttributesRequest{
		ContactListId: id,
		ClientId:      clientID.String(),
		Attributes:    req.Attributes,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetListAttributes обрабатывает GET /contact-lists/{id}/attributes
func (h *ContactHandlers) GetListAttributes(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.contactClient.GetListAttributes(r.Context(), &contactv1.GetListAttributesRequest{
		ContactListId: id,
		ClientId:      clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// CreateContact обрабатывает POST /contact-lists/{id}/contacts
func (h *ContactHandlers) CreateContact(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]

	// Use plain Go struct to decode — encoding/json cannot deserialize into google.protobuf.Struct
	var body struct {
		Phone      string                 `json:"phone"`
		Attributes map[string]interface{} `json:"attributes"`
		Tags       []string               `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &contactv1.CreateContactRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
		Phone:         body.Phone,
		Tags:          body.Tags,
	}

	if len(body.Attributes) > 0 {
		protoStruct, err := convertMapToStruct(body.Attributes)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат атрибутов"))
			return
		}
		grpcReq.Attributes = protoStruct
	}

	resp, err := h.contactClient.CreateContact(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// ListContacts обрабатывает GET /contact-lists/{id}/contacts
func (h *ContactHandlers) ListContacts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	search := r.URL.Query().Get("search")

	var tags []string
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
	}

	resp, err := h.contactClient.ListContacts(r.Context(), &contactv1.ListContactsRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
		Limit:         perPage,
		Offset:        offset,
		Search:        search,
		Tags:          tags,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// UpdateContact обрабатывает PUT /contact-lists/{id}/contacts/{cid}
func (h *ContactHandlers) UpdateContact(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	vars := mux.Vars(r)
	listID := vars["id"]
	contactID := vars["cid"]

	// Use plain Go struct to decode — encoding/json cannot deserialize into google.protobuf.Struct
	var body struct {
		Phone      string                 `json:"phone"`
		Attributes map[string]interface{} `json:"attributes"`
		Tags       []string               `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &contactv1.UpdateContactRequest{
		Id:            contactID,
		ContactListId: listID,
		ClientId:      clientID.String(),
		Phone:         body.Phone,
		Tags:          body.Tags,
	}

	if len(body.Attributes) > 0 {
		protoStruct, err := convertMapToStruct(body.Attributes)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат атрибутов"))
			return
		}
		grpcReq.Attributes = protoStruct
	}

	resp, err := h.contactClient.UpdateContact(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// DeleteContact обрабатывает DELETE /contact-lists/{id}/contacts/{cid}
func (h *ContactHandlers) DeleteContact(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	vars := mux.Vars(r)
	listID := vars["id"]
	contactID := vars["cid"]

	_, err := h.contactClient.DeleteContact(r.Context(), &contactv1.DeleteContactRequest{
		Id:            contactID,
		ContactListId: listID,
		ClientId:      clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BatchUpsertContacts обрабатывает POST /contact-lists/{id}/contacts/batch
func (h *ContactHandlers) BatchUpsertContacts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]

	var req struct {
		Contacts []*contactv1.ContactInput `json:"contacts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.contactClient.BatchUpsertContacts(r.Context(), &contactv1.BatchUpsertContactsRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
		Contacts:      req.Contacts,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// AddTags обрабатывает POST /contact-lists/{id}/contacts/tags
func (h *ContactHandlers) AddTags(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]

	var req struct {
		ContactIds []string `json:"contact_ids"`
		Tags       []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	_, err := h.contactClient.AddTags(r.Context(), &contactv1.AddTagsRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
		ContactIds:    req.ContactIds,
		Tags:          req.Tags,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveTags обрабатывает DELETE /contact-lists/{id}/contacts/tags
func (h *ContactHandlers) RemoveTags(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]

	var req struct {
		ContactIds []string `json:"contact_ids"`
		Tags       []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	_, err := h.contactClient.RemoveTags(r.Context(), &contactv1.RemoveTagsRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
		ContactIds:    req.ContactIds,
		Tags:          req.Tags,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListTags обрабатывает GET /contact-lists/{id}/tags
func (h *ContactHandlers) ListTags(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]

	resp, err := h.contactClient.ListTags(r.Context(), &contactv1.ListTagsRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// UploadImport обрабатывает POST /contact-lists/{id}/imports/upload
func (h *ContactHandlers) UploadImport(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]

	// Ограничиваем размер файла — 50MB
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, shared.ErrInvalidInput("Ошибка разбора multipart формы"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Файл не найден в запросе"))
		return
	}
	defer file.Close()

	importID := uuid.New().String()

	// Создаём директорию для загрузки
	uploadDir := filepath.Join("uploads", clientID.String(), importID)
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Error().Err(err).Msg("ошибка создания директории для импорта")
		respondError(w, shared.ErrInternalServer("Ошибка сохранения файла"))
		return
	}

	// Сохраняем файл
	destPath := filepath.Join(uploadDir, header.Filename)
	dest, err := os.Create(destPath)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания файла для импорта")
		respondError(w, shared.ErrInternalServer("Ошибка сохранения файла"))
		return
	}
	defer dest.Close()

	written, err := io.Copy(dest, file)
	if err != nil {
		log.Error().Err(err).Msg("ошибка записи файла для импорта")
		respondError(w, shared.ErrInternalServer("Ошибка сохранения файла"))
		return
	}

	// Парсим превью первых 5 строк
	preview := parseFilePreview(destPath, 5)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"import_id":       importID,
		"contact_list_id": listID,
		"file_name":       header.Filename,
		"file_size":       written,
		"preview":         preview,
	})
}

// parseFilePreview читает превью из CSV или XLSX файла.
func parseFilePreview(filePath string, maxRows int) [][]string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".xlsx" || ext == ".xls" {
		return parseXLSXPreview(filePath, maxRows)
	}
	return parseCSVPreview(filePath, maxRows)
}

// parseXLSXPreview читает первые N строк XLSX файла для превью.
func parseXLSXPreview(filePath string, maxRows int) [][]string {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil
	}
	n := maxRows + 1 // включая заголовок
	if len(rows) < n {
		n = len(rows)
	}
	return rows[:n]
}

// parseCSVPreview читает первые N строк CSV файла для превью
func parseCSVPreview(filePath string, maxRows int) [][]string {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	reader := csv.NewReader(bufio.NewReader(f))
	var rows [][]string
	for i := 0; i <= maxRows; i++ { // +1 для заголовка
		record, err := reader.Read()
		if err != nil {
			break
		}
		rows = append(rows, record)
	}
	return rows
}

// StartImport обрабатывает POST /contact-lists/{id}/imports/{iid}/start
func (h *ContactHandlers) StartImport(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	vars := mux.Vars(r)
	listID := vars["id"]
	importID := vars["iid"]

	// column_mapping: { "phone": 0, "attr:Name": 1, "tags": 2 }
	var req struct {
		ColumnMapping map[string]int `json:"column_mapping"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Convert frontend format to domain []ColumnMapping JSON string
	type columnTarget struct {
		Field string `json:"field"`
		Type  string `json:"type"`
	}
	type columnMapping struct {
		Column int          `json:"column"`
		Target columnTarget `json:"target"`
	}
	var domainMapping []columnMapping
	for field, colIdx := range req.ColumnMapping {
		var target columnTarget
		switch {
		case field == "phone":
			target = columnTarget{Field: "phone", Type: "phone"}
		case field == "tags":
			target = columnTarget{Field: "tags", Type: "tag"}
		case len(field) > 5 && field[:5] == "attr:":
			target = columnTarget{Field: field[5:], Type: "attribute"}
		default:
			target = columnTarget{Field: field, Type: "attribute"}
		}
		domainMapping = append(domainMapping, columnMapping{Column: colIdx, Target: target})
	}
	mappingJSON, err := json.Marshal(domainMapping)
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка сериализации маппинга"))
		return
	}
	columnMappingStr := string(mappingJSON)

	// Определяем имя и размер файла из директории загрузки
	uploadDir := filepath.Join("uploads", clientID.String(), importID)
	entries, err := os.ReadDir(uploadDir)
	if err != nil || len(entries) == 0 {
		respondError(w, shared.ErrNotFound("Файл импорта не найден"))
		return
	}
	fileName := entries[0].Name()
	fileInfo, _ := entries[0].Info()
	var fileSize int64
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	resp, err := h.contactClient.StartImport(r.Context(), &contactv1.StartImportRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
		ImportId:      importID,
		FileName:      fileName,
		FileSize:      fileSize,
		ColumnMapping: columnMappingStr,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetImportStatus обрабатывает GET /contact-lists/{id}/imports/{iid}
func (h *ContactHandlers) GetImportStatus(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	vars := mux.Vars(r)
	listID := vars["id"]
	importID := vars["iid"]

	resp, err := h.contactClient.GetImportStatus(r.Context(), &contactv1.GetImportStatusRequest{
		Id:            importID,
		ContactListId: listID,
		ClientId:      clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// ListImports обрабатывает GET /contact-lists/{id}/imports
func (h *ContactHandlers) ListImports(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage

	resp, err := h.contactClient.ListImports(r.Context(), &contactv1.ListImportsRequest{
		ContactListId: listID,
		ClientId:      clientID.String(),
		Limit:         perPage,
		Offset:        offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// PreviewSegment обрабатывает POST /contact-lists/{id}/segment/preview
func (h *ContactHandlers) PreviewSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	listID := mux.Vars(r)["id"]

	var req contactv1.PreviewSegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ContactListId = listID
	req.ClientId = clientID.String()

	resp, err := h.contactClient.PreviewSegment(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetContactListSegments обрабатывает GET /contact-lists/{id}/segments
// Возвращает уникальные страны и операторов в базе контактов по префиксам номеров.
func (h *ContactHandlers) GetContactListSegments(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	// Проверяем что база принадлежит клиенту
	_, err := h.contactClient.GetContactList(r.Context(), &contactv1.GetContactListRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	// Возвращаем список стран и операторов для фильтрации
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"countries": []map[string]string{
			{"code": "RU", "name": "Россия"},
			{"code": "KZ", "name": "Казахстан"},
			{"code": "BY", "name": "Беларусь"},
			{"code": "UA", "name": "Украина"},
			{"code": "UZ", "name": "Узбекистан"},
			{"code": "KG", "name": "Кыргызстан"},
			{"code": "TJ", "name": "Таджикистан"},
			{"code": "TM", "name": "Туркменистан"},
			{"code": "AM", "name": "Армения"},
			{"code": "AZ", "name": "Азербайджан"},
			{"code": "GE", "name": "Грузия"},
			{"code": "MD", "name": "Молдова"},
		},
		"operators": []map[string]string{
			{"code": "mts", "name": "МТС"},
			{"code": "beeline", "name": "Билайн"},
			{"code": "megafon", "name": "МегаФон"},
			{"code": "tele2", "name": "Tele2"},
			{"code": "other", "name": "Другие"},
		},
	})
}
