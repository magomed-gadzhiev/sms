package shared

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// NullString represents a string that may be null, scanning nulls to empty string.
type NullString string

func (ns *NullString) Scan(value interface{}) error {
	if value == nil {
		*ns = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*ns = NullString(v)
	case []byte:
		*ns = NullString(string(v))
	}
	return nil
}

func (ns NullString) Value() (driver.Value, error) {
	if ns == "" {
		return nil, nil // Write empty string as NULL to the DB
	}
	return string(ns), nil
}

// StringArray представляет массив строк для PostgreSQL
type StringArray []string

// Scan реализует интерфейс sql.Scanner для сканирования PostgreSQL массива
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = StringArray{}
		return nil
	}

	// Обрабатываем разные типы, которые могут вернуть драйверы
	switch v := value.(type) {
	case []byte:
		return a.scanBytes(v)
	case string:
		return a.scanBytes([]byte(v))
	case pq.StringArray:
		*a = StringArray(v)
		return nil
	case []string:
		*a = StringArray(v)
		return nil
	default:
		// Для pgx драйвера массив может прийти как []interface{}
		if arr, ok := value.([]interface{}); ok {
			result := make(StringArray, len(arr))
			for i, item := range arr {
				if str, ok := item.(string); ok {
					result[i] = str
				} else {
					result[i] = ""
				}
			}
			*a = result
			return nil
		}
		
		// Пробуем распарсить как JSON
		if bytes, ok := value.([]byte); ok {
			var arr []string
			if err := json.Unmarshal(bytes, &arr); err == nil {
				*a = StringArray(arr)
				return nil
			}
		}
		
		// Пробуем распарсить как строку PostgreSQL массива
		if str, ok := value.(string); ok {
			return a.scanBytes([]byte(str))
		}
	}

	*a = StringArray{}
	return nil
}

// scanBytes сканирует байты PostgreSQL массива
func (a *StringArray) scanBytes(src []byte) error {
	if len(src) == 0 {
		*a = StringArray{}
		return nil
	}

	// Парсим PostgreSQL массив формата {val1,val2,val3}
	str := string(src)
	if strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
		str = str[1 : len(str)-1]
		if str == "" {
			*a = StringArray{}
			return nil
		}
		parts := strings.Split(str, ",")
		result := make(StringArray, len(parts))
		for i, part := range parts {
			// Убираем кавычки если есть
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`) {
				part = part[1 : len(part)-1]
			}
			result[i] = part
		}
		*a = result
		return nil
	}

	// Если не массив, пробуем как JSON
	var arr []string
	if err := json.Unmarshal(src, &arr); err == nil {
		*a = StringArray(arr)
		return nil
	}

	*a = StringArray{}
	return nil
}

// Value реализует интерфейс driver.Valuer для сохранения в базу данных
func (a StringArray) Value() (driver.Value, error) {
	if a == nil || len(a) == 0 {
		return pq.Array([]string{}).Value()
	}
	return pq.Array([]string(a)).Value()
}

// Provider представляет SMSC провайдера
type Provider struct {
	ID                uuid.UUID `json:"id" db:"id"`
	Name              string    `json:"name" db:"name"`
	Host              string    `json:"host" db:"host"`
	Port              int       `json:"port" db:"port"`
	SystemID          string    `json:"system_id" db:"system_id"`
	Password          string    `json:"password" db:"password"`
	SystemType        string    `json:"system_type" db:"system_type"`
	BindType          string    `json:"bind_type" db:"bind_type"` // transceiver, transmitter, receiver
	BindTON           int       `json:"bind_ton" db:"bind_ton"`
	BindNPI           int       `json:"bind_npi" db:"bind_npi"`
	AddrTON           int       `json:"addr_ton" db:"addr_ton"`
	AddrNPI           int       `json:"addr_npi" db:"addr_npi"`
	AddressRange      string    `json:"address_range" db:"address_range"`
	MaxConnections    int       `json:"max_connections" db:"max_connections"`
	Active            bool      `json:"active" db:"active"`
	Priority          int       `json:"priority" db:"priority"`
	ThroughputPerSec  int       `json:"throughput_per_second" db:"throughput_per_second"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
	// Поля для client-провайдеров
	ClientID     *uuid.UUID      `json:"client_id,omitempty" db:"client_id"`
	Description  string          `json:"description" db:"description"`
	Tags         StringArray     `json:"tags" db:"tags"`
	TPSLimit     int             `json:"tps_limit" db:"tps_limit"`
	RoutingRules json.RawMessage `json:"routing_rules" db:"routing_rules"`
	DailyQuota   *int            `json:"daily_quota,omitempty" db:"daily_quota"`
	MonthlyQuota *int            `json:"monthly_quota,omitempty" db:"monthly_quota"`
}

// Route представляет правило маршрутизации
type Route struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	Name             string     `json:"name" db:"name"`
	Pattern          string     `json:"pattern" db:"pattern"`
	PatternType      string     `json:"pattern_type" db:"pattern_type"` // prefix, regex, exact
	ProviderID       uuid.UUID  `json:"provider_id" db:"provider_id"`
	Priority         int        `json:"priority" db:"priority"`
	Active           bool       `json:"active" db:"active"`
	FailoverProviderID *uuid.UUID `json:"failover_provider_id,omitempty" db:"failover_provider_id"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// Client представляет клиента API
type Client struct {
	ID                   uuid.UUID    `json:"id" db:"id"`
	Name                 string       `json:"name" db:"name"`
	APIKey               string       `json:"api_key" db:"api_key"`
	Secret               string       `json:"secret" db:"secret"`
	Active               bool         `json:"active" db:"active"`
	RateLimitPerSecond   int          `json:"rate_limit_per_second" db:"rate_limit_per_second"`
	RateLimitPerMinute   int          `json:"rate_limit_per_minute" db:"rate_limit_per_minute"`
	RateLimitPerHour     int          `json:"rate_limit_per_hour" db:"rate_limit_per_hour"`
	AllowedSourceAddresses StringArray `json:"allowed_source_addresses" db:"allowed_source_addresses"`
	CreatedAt            time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at" db:"updated_at"`
	RoutingMode           string       `json:"routing_mode" db:"routing_mode"`           // "legacy", "new", "hybrid"
	CostVisibilityEnabled bool         `json:"cost_visibility_enabled" db:"cost_visibility_enabled"`
}

// MessageStatus представляет статус сообщения
type MessageStatus string

const (
	MessageStatusPending   MessageStatus = "pending"
	MessageStatusQueued    MessageStatus = "queued"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusFailed    MessageStatus = "failed"
	MessageStatusExpired   MessageStatus = "expired"
	MessageStatusRejected  MessageStatus = "rejected"
	MessageStatusScheduled MessageStatus = "scheduled"
	MessageStatusCancelled MessageStatus = "cancelled"
)

// MessageFilter задаёт фильтры для выборки сообщений
type MessageFilter struct {
	Status      *MessageStatus
	Destination string
	From        *time.Time
	To          *time.Time
	Limit       int
	Offset      int
}

// MessageEncoding представляет кодировку сообщения
type MessageEncoding string

const (
	MessageEncodingGSM7  MessageEncoding = "GSM7"
	MessageEncodingUCS2  MessageEncoding = "UCS2"
	MessageEncodingASCII MessageEncoding = "ASCII"
)

// Message представляет SMS сообщение
type Message struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	MessageID      NullString    `json:"message_id,omitempty" db:"message_id"`
	ExternalID     NullString    `json:"external_id,omitempty" db:"external_id"`
	Source         string        `json:"source" db:"source" `
	Destination    string        `json:"destination" db:"destination"`
	Text           string        `json:"text" db:"text"`
	Encoding       MessageEncoding `json:"encoding" db:"encoding"`
	DataCoding     int           `json:"data_coding" db:"data_coding"`
	ESMClass       int           `json:"esm_class" db:"esm_class"`
	ProtocolID     int           `json:"protocol_id" db:"protocol_id"`
	PriorityFlag   int           `json:"priority_flag" db:"priority_flag"`
	ReplaceIfPresent int         `json:"replace_if_present" db:"replace_if_present"`
	RegisteredDelivery int       `json:"registered_delivery" db:"registered_delivery"`
	ValidityPeriod *time.Time    `json:"validity_period,omitempty" db:"validity_period"`
	ServiceType    string        `json:"service_type" db:"service_type"`
	SourceAddrTON  int           `json:"source_addr_ton" db:"source_addr_ton"`
	SourceAddrNPI  int           `json:"source_addr_npi" db:"source_addr_npi"`
	DestAddrTON    int           `json:"dest_addr_ton" db:"dest_addr_ton"`
	DestAddrNPI    int           `json:"dest_addr_npi" db:"dest_addr_npi"`
	Status         MessageStatus `json:"status" db:"status"`
	StatusMessage  NullString    `json:"status_message,omitempty" db:"status_message"`
	ProviderID     *uuid.UUID    `json:"provider_id,omitempty" db:"provider_id"`
	RouteID        *uuid.UUID    `json:"route_id,omitempty" db:"route_id"`
	ClientID       *uuid.UUID    `json:"client_id,omitempty" db:"client_id"`
	RetryCount     int           `json:"retry_count" db:"retry_count"`
	MaxRetries     int           `json:"max_retries" db:"max_retries"`
	NextRetryAt    *time.Time    `json:"next_retry_at,omitempty" db:"next_retry_at"`
	SMPPMessageID  NullString    `json:"smpp_message_id,omitempty" db:"smpp_message_id"`
	SubmittedAt    *time.Time    `json:"submitted_at,omitempty" db:"submitted_at"`
	DeliveredAt    *time.Time    `json:"delivered_at,omitempty" db:"delivered_at"`
	FailedAt       *time.Time    `json:"failed_at,omitempty" db:"failed_at"`
	ScheduledAt    *time.Time    `json:"scheduled_at,omitempty" db:"scheduled_at"`
	ExpiredAt      *time.Time    `json:"expired_at,omitempty" db:"expired_at"`
	SegmentCount   int           `json:"segment_count" db:"segment_count"`
	Channel        NullString    `json:"channel,omitempty" db:"channel"`
	OperatorID     *uuid.UUID    `json:"operator_id,omitempty" db:"operator_id"`
	CountryID      *uuid.UUID    `json:"country_id,omitempty" db:"country_id"`
	SendMethod     NullString    `json:"send_method,omitempty" db:"send_method"`
	TemplateID     *uuid.UUID    `json:"template_id,omitempty" db:"template_id"`
	SenderNameID   *uuid.UUID    `json:"sender_name_id,omitempty" db:"sender_name_id"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
}

// DLRReceipt представляет delivery receipt от SMSC
type DLRReceipt struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	MessageID       uuid.UUID  `json:"message_id" db:"message_id"`
	SMPPMessageID   string     `json:"smpp_message_id" db:"smpp_message_id"`
	ProviderID      *uuid.UUID `json:"provider_id,omitempty" db:"provider_id"`
	ReceiptedMessageID string  `json:"receipted_message_id,omitempty" db:"receipted_message_id"`
	SubmitDate      *time.Time `json:"submit_date,omitempty" db:"submit_date"`
	DoneDate        *time.Time `json:"done_date,omitempty" db:"done_date"`
	Stat            string     `json:"stat" db:"stat"`
	Err             *int       `json:"err,omitempty" db:"err"`
	Text            string     `json:"text,omitempty" db:"text"`
	Source          string     `json:"source,omitempty" db:"source"`
	Destination     string     `json:"destination,omitempty" db:"destination"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
}

// AuditLogAction представляет действие в аудит логе
type AuditLogAction string

const (
	AuditActionCreate AuditLogAction = "create"
	AuditActionUpdate AuditLogAction = "update"
	AuditActionDelete AuditLogAction = "delete"
	AuditActionSend   AuditLogAction = "send"
	AuditActionDeliver AuditLogAction = "deliver"
	AuditActionFail   AuditLogAction = "fail"
)

// AuditLogEntry представляет запись в аудит логе
type AuditLogEntry struct {
	ID         uuid.UUID              `json:"id" db:"id"`
	EntityType string                 `json:"entity_type" db:"entity_type"`
	EntityID   uuid.UUID              `json:"entity_id" db:"entity_id"`
	Action     AuditLogAction         `json:"action" db:"action"`
	ActorType  string                 `json:"actor_type,omitempty" db:"actor_type"`
	ActorID    *uuid.UUID             `json:"actor_id,omitempty" db:"actor_id"`
	Details    map[string]interface{} `json:"details,omitempty" db:"details"`
	IPAddress  string                 `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  string                 `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
}

// ClientRoute — маршрут клиента к провайдеру через оператора.
// client_id = NULL означает платформенный дефолт.
type ClientRoute struct {
	ID         uuid.UUID  `db:"id"`
	ClientID   *uuid.UUID `db:"client_id"`
	OperatorID uuid.UUID  `db:"operator_id"`
	ProviderID uuid.UUID  `db:"provider_id"`
	Priority   int        `db:"priority"`
	Weight     int        `db:"weight"`
	Active     bool       `db:"active"`
	Shared     bool       `db:"shared"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

// OperatorPrefix — префикс оператора для определения оператора по номеру.
// CountryID заполняется JOIN'ом на operators и может быть nil, если оператор
// в БД не имеет country_id.
type OperatorPrefix struct {
	OperatorID uuid.UUID
	CountryID  *uuid.UUID
	Prefix     string
	Priority   int
}

// RoutingDecision — результат маршрутизации.
type RoutingDecision struct {
	ProviderID uuid.UUID
	RouteID    uuid.UUID
}

// Company представляет юридическое лицо клиента
type Company struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	INN             NullString `json:"inn,omitempty" db:"inn"`
	Name            string     `json:"name" db:"name"`
	FullName        NullString `json:"full_name,omitempty" db:"full_name"`
	KPP             NullString `json:"kpp,omitempty" db:"kpp"`
	OGRN            NullString `json:"ogrn,omitempty" db:"ogrn"`
	LegalAddress    NullString `json:"legal_address,omitempty" db:"legal_address"`
	ActualAddress   NullString `json:"actual_address,omitempty" db:"actual_address"`
	CEOName         NullString `json:"ceo_name,omitempty" db:"ceo_name"`
	CEOTitle        NullString `json:"ceo_title,omitempty" db:"ceo_title"`
	ActingBasis     NullString `json:"acting_basis,omitempty" db:"acting_basis"`
	BankName        NullString `json:"bank_name,omitempty" db:"bank_name"`
	BankBIK         NullString `json:"bank_bik,omitempty" db:"bank_bik"`
	BankCorrAccount NullString `json:"bank_corr_account,omitempty" db:"bank_corr_account"`
	BankAccount     NullString `json:"bank_account,omitempty" db:"bank_account"`
	Email           NullString `json:"email,omitempty" db:"email"`
	Phone           NullString `json:"phone,omitempty" db:"phone"`
	IsOffer         bool       `json:"is_offer" db:"is_offer"`
	Active          bool       `json:"active" db:"active"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}
