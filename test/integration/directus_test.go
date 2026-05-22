//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DirectusClient представляет клиент для работы с Directus API
type DirectusClient struct {
	BaseURL     string
	AccessToken string
	HTTPClient  *http.Client
}

// NewDirectusClient создает новый клиент Directus
func NewDirectusClient(baseURL string) *DirectusClient {
	return &DirectusClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Login выполняет аутентификацию в Directus
func (c *DirectusClient) Login(email, password string) error {
	url := fmt.Sprintf("%s/auth/login", c.BaseURL)
	
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal login payload: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d", resp.StatusCode)
	}
	
	var result struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	c.AccessToken = result.Data.AccessToken
	return nil
}

// CreateItem создает новый элемент в коллекции
func (c *DirectusClient) CreateItem(collection string, item interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/items/%s", c.BaseURL, collection)
	
	jsonData, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal item: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken))
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorBody)
		return nil, fmt.Errorf("create failed with status %d: %v", resp.StatusCode, errorBody)
	}
	
	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.Data, nil
}

// GetItem получает элемент из коллекции по ID
func (c *DirectusClient) GetItem(collection, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/items/%s/%s", c.BaseURL, collection, id)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken))
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorBody)
		return nil, fmt.Errorf("get failed with status %d: %v", resp.StatusCode, errorBody)
	}
	
	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.Data, nil
}

// UpdateItem обновляет элемент в коллекции
func (c *DirectusClient) UpdateItem(collection, id string, item interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/items/%s/%s", c.BaseURL, collection, id)
	
	jsonData, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal item: %w", err)
	}
	
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken))
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		var errorBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorBody)
		return nil, fmt.Errorf("update failed with status %d: %v", resp.StatusCode, errorBody)
	}
	
	// Если ответ 204 No Content, возвращаем пустой результат
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	
	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.Data, nil
}

// DeleteItem удаляет элемент из коллекции
func (c *DirectusClient) DeleteItem(collection, id string) error {
	url := fmt.Sprintf("%s/items/%s/%s", c.BaseURL, collection, id)
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken))
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		var errorBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorBody)
		return fmt.Errorf("delete failed with status %d: %v", resp.StatusCode, errorBody)
	}
	
	return nil
}

// ListItems получает список элементов из коллекции
func (c *DirectusClient) ListItems(collection string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/items/%s", c.BaseURL, collection)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.AccessToken))
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorBody)
		return nil, fmt.Errorf("list failed with status %d: %v", resp.StatusCode, errorBody)
	}
	
	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.Data, nil
}

// GetDirectusConfig получает конфигурацию Directus из переменных окружения
func GetDirectusConfig() (baseURL, email, password string) {
	baseURL = os.Getenv("DIRECTUS_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8055"
	}
	
	email = os.Getenv("DIRECTUS_ADMIN_EMAIL")
	if email == "" {
		email = "admin@example.com"
	}
	
	password = os.Getenv("DIRECTUS_ADMIN_PASSWORD")
	if password == "" {
		password = "admin"
	}
	
	return baseURL, email, password
}

// TestDirectusHealth проверяет доступность Directus
func TestDirectusHealth(t *testing.T) {
	baseURL, _, _ := GetDirectusConfig()
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/server/health", baseURL))
	if err != nil {
		t.Skipf("Directus недоступен: %v", err)
		return
	}
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Directus должен быть доступен")
}

// TestDirectusClientCRUD тестирует CRUD операции для коллекции clients
func TestDirectusClientCRUD(t *testing.T) {
	baseURL, email, password := GetDirectusConfig()
	
	client := NewDirectusClient(baseURL)
	
	// Аутентификация
	err := client.Login(email, password)
	require.NoError(t, err, "Должна быть успешная аутентификация")
	assert.NotEmpty(t, client.AccessToken, "Access token должен быть получен")
	
	// CREATE: Создание нового клиента
	testClientID := uuid.New().String()
	clientData := map[string]interface{}{
		"id":                      testClientID,
		"name":                    "Test Client from Directus",
		"api_key":                 fmt.Sprintf("test-key-%s", uuid.New().String()),
		"secret":                  "test-secret-12345",
		"active":                  true,
		"rate_limit_per_second":   20,
		"rate_limit_per_minute":   200,
		"rate_limit_per_hour":     2000,
		"allowed_source_addresses": []string{"12345", "67890"},
	}
	
	created, err := client.CreateItem("clients", clientData)
	require.NoError(t, err, "Создание клиента должно быть успешным")
	assert.NotNil(t, created, "Созданный клиент не должен быть nil")
	assert.Equal(t, clientData["name"], created["name"], "Имя клиента должно совпадать")
	
	createdID, ok := created["id"].(string)
	require.True(t, ok, "ID должен быть строкой")
	
	// READ: Получение созданного клиента
	retrieved, err := client.GetItem("clients", createdID)
	require.NoError(t, err, "Получение клиента должно быть успешным")
	assert.Equal(t, created["name"], retrieved["name"], "Имя должно совпадать")
	assert.Equal(t, created["api_key"], retrieved["api_key"], "API ключ должен совпадать")
	
	// UPDATE: Обновление клиента
	updateData := map[string]interface{}{
		"name":                  "Updated Test Client",
		"rate_limit_per_second": 30,
	}
	
	updated, err := client.UpdateItem("clients", createdID, updateData)
	require.NoError(t, err, "Обновление клиента должно быть успешным")
	
	// Проверяем обновление
	updatedRetrieved, err := client.GetItem("clients", createdID)
	require.NoError(t, err, "Получение обновленного клиента должно быть успешным")
	assert.Equal(t, "Updated Test Client", updatedRetrieved["name"], "Имя должно быть обновлено")
	if updated != nil {
		// Если API возвращает обновленные данные, проверяем их
		assert.Equal(t, "Updated Test Client", updated["name"], "Имя в ответе должно быть обновлено")
	}
	
	// DELETE: Удаление клиента
	err = client.DeleteItem("clients", createdID)
	require.NoError(t, err, "Удаление клиента должно быть успешным")
	
	// Проверяем, что клиент удален (попытка получения должна вернуть ошибку)
	_, err = client.GetItem("clients", createdID)
	assert.Error(t, err, "Получение удаленного клиента должно вернуть ошибку")
}

// TestDirectusProviderCRUD тестирует CRUD операции для коллекции providers
func TestDirectusProviderCRUD(t *testing.T) {
	baseURL, email, password := GetDirectusConfig()
	
	client := NewDirectusClient(baseURL)
	
	// Аутентификация
	err := client.Login(email, password)
	require.NoError(t, err, "Должна быть успешная аутентификация")
	
	// CREATE: Создание нового провайдера
	testProviderID := uuid.New().String()
	providerData := map[string]interface{}{
		"id":                   testProviderID,
		"name":                 "Test Provider from Directus",
		"host":                 "test.example.com",
		"port":                 2775,
		"system_id":            "test-system-id",
		"password":             "test-password",
		"system_type":          "CMT",
		"bind_type":            "transceiver",
		"bind_ton":             0,
		"bind_npi":             0,
		"addr_ton":             0,
		"addr_npi":             0,
		"address_range":        "",
		"max_connections":      10,
		"active":               true,
		"priority":             1,
		"throughput_per_second": 100,
	}
	
	created, err := client.CreateItem("providers", providerData)
	require.NoError(t, err, "Создание провайдера должно быть успешным")
	assert.NotNil(t, created, "Созданный провайдер не должен быть nil")
	
	createdID, ok := created["id"].(string)
	require.True(t, ok, "ID должен быть строкой")
	
	// READ: Получение созданного провайдера
	retrieved, err := client.GetItem("providers", createdID)
	require.NoError(t, err, "Получение провайдера должно быть успешным")
	assert.Equal(t, created["name"], retrieved["name"], "Имя должно совпадать")
	assert.Equal(t, created["host"], retrieved["host"], "Хост должен совпадать")
	
	// UPDATE: Обновление провайдера
	updateData := map[string]interface{}{
		"name":   "Updated Test Provider",
		"host":   "updated.example.com",
		"active": false,
	}
	
	_, err = client.UpdateItem("providers", createdID, updateData)
	require.NoError(t, err, "Обновление провайдера должно быть успешным")
	
	// Проверяем обновление
	updatedRetrieved, err := client.GetItem("providers", createdID)
	require.NoError(t, err, "Получение обновленного провайдера должно быть успешным")
	assert.Equal(t, "Updated Test Provider", updatedRetrieved["name"], "Имя должно быть обновлено")
	assert.Equal(t, "updated.example.com", updatedRetrieved["host"], "Хост должен быть обновлен")
	
	// DELETE: Удаление провайдера
	err = client.DeleteItem("providers", createdID)
	require.NoError(t, err, "Удаление провайдера должно быть успешным")
	
	// Проверяем, что провайдер удален
	_, err = client.GetItem("providers", createdID)
	assert.Error(t, err, "Получение удаленного провайдера должно вернуть ошибку")
}

// TestDirectusRouteCRUD тестирует CRUD операции для коллекции routes
// Примечание: routes требует существующий provider_id
func TestDirectusRouteCRUD(t *testing.T) {
	baseURL, email, password := GetDirectusConfig()
	
	client := NewDirectusClient(baseURL)
	
	// Аутентификация
	err := client.Login(email, password)
	require.NoError(t, err, "Должна быть успешная аутентификация")
	
	// Сначала создаем провайдера, чтобы использовать его ID для маршрута
	providerData := map[string]interface{}{
		"id":                   uuid.New().String(),
		"name":                 "Test Provider for Route",
		"host":                 "route-provider.example.com",
		"port":                 2775,
		"system_id":            "route-system-id",
		"password":             "route-password",
		"system_type":          "CMT",
		"bind_type":            "transceiver",
		"bind_ton":             0,
		"bind_npi":             0,
		"addr_ton":             0,
		"addr_npi":             0,
		"address_range":        "",
		"max_connections":      5,
		"active":               true,
		"priority":             1,
		"throughput_per_second": 50,
	}
	
	createdProvider, err := client.CreateItem("providers", providerData)
	require.NoError(t, err, "Создание провайдера для маршрута должно быть успешным")
	
	providerID, ok := createdProvider["id"].(string)
	require.True(t, ok, "ID провайдера должен быть строкой")
	
	// CREATE: Создание нового маршрута
	testRouteID := uuid.New().String()
	routeData := map[string]interface{}{
		"id":          testRouteID,
		"name":        "Test Route from Directus",
		"pattern":     "7900",
		"pattern_type": "prefix",
		"provider_id": providerID,
		"priority":    1,
		"active":      true,
	}
	
	created, err := client.CreateItem("routes", routeData)
	require.NoError(t, err, "Создание маршрута должно быть успешным")
	assert.NotNil(t, created, "Созданный маршрут не должен быть nil")
	
	createdID, ok := created["id"].(string)
	require.True(t, ok, "ID должен быть строкой")
	
	// READ: Получение созданного маршрута
	retrieved, err := client.GetItem("routes", createdID)
	require.NoError(t, err, "Получение маршрута должно быть успешным")
	assert.Equal(t, created["name"], retrieved["name"], "Имя должно совпадать")
	assert.Equal(t, created["pattern"], retrieved["pattern"], "Паттерн должен совпадать")
	
	// UPDATE: Обновление маршрута
	updateData := map[string]interface{}{
		"name":        "Updated Test Route",
		"pattern":     "7910",
		"active":      false,
	}
	
	_, err = client.UpdateItem("routes", createdID, updateData)
	require.NoError(t, err, "Обновление маршрута должно быть успешным")
	
	// Проверяем обновление
	updatedRetrieved, err := client.GetItem("routes", createdID)
	require.NoError(t, err, "Получение обновленного маршрута должно быть успешным")
	assert.Equal(t, "Updated Test Route", updatedRetrieved["name"], "Имя должно быть обновлено")
	assert.Equal(t, "7910", updatedRetrieved["pattern"], "Паттерн должен быть обновлен")
	
	// DELETE: Удаление маршрута
	err = client.DeleteItem("routes", createdID)
	require.NoError(t, err, "Удаление маршрута должно быть успешным")
	
	// Удаляем также тестового провайдера
	err = client.DeleteItem("providers", providerID)
	require.NoError(t, err, "Удаление тестового провайдера должно быть успешным")
	
	// Проверяем, что маршрут удален
	_, err = client.GetItem("routes", createdID)
	assert.Error(t, err, "Получение удаленного маршрута должно вернуть ошибку")
}

// TestDirectusListItems тестирует получение списка элементов
func TestDirectusListItems(t *testing.T) {
	baseURL, email, password := GetDirectusConfig()
	
	client := NewDirectusClient(baseURL)
	
	// Аутентификация
	err := client.Login(email, password)
	require.NoError(t, err, "Должна быть успешная аутентификация")
	
	// Получаем список клиентов
	clients, err := client.ListItems("clients")
	require.NoError(t, err, "Получение списка клиентов должно быть успешным")
	assert.IsType(t, []map[string]interface{}{}, clients, "Результат должен быть массивом")
	
	// Получаем список провайдеров
	providers, err := client.ListItems("providers")
	require.NoError(t, err, "Получение списка провайдеров должно быть успешным")
	assert.IsType(t, []map[string]interface{}{}, providers, "Результат должен быть массивом")
	
	// Получаем список маршрутов
	routes, err := client.ListItems("routes")
	require.NoError(t, err, "Получение списка маршрутов должно быть успешным")
	assert.IsType(t, []map[string]interface{}{}, routes, "Результат должен быть массивом")
}
