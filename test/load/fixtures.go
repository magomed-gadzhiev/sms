//go:build load

package load

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"
)

// ============================================================
// Пулы реалистичных данных для нагрузочного тестирования
// ============================================================

// Российские мобильные префиксы по операторам
var mobileDestPrefixes = []string{
	// MTS
	"7910", "7911", "7912", "7913", "7914", "7915", "7916", "7917", "7918", "7919", "7985", "7986",
	// Beeline
	"7903", "7905", "7906", "7909", "7960", "7961", "7962", "7963", "7964", "7965",
	// Megafon
	"7920", "7921", "7922", "7923", "7924", "7925", "7926", "7927", "7928", "7929",
	// Tele2
	"7900", "7901", "7902", "7904", "7908", "7950", "7951", "7952", "7953",
	// Tinkoff / MVNO
	"7999", "7977", "7978",
}

// Международные префиксы (CIS)
var intlDestPrefixes = []string{
	"37529", "37533", "37544", // Беларусь
	"7701", "7702", "7705", "7707", "7708", // Казахстан
}

// Source-адреса (имена отправителей)
var sourceAddresses = []string{
	"LoadTest", "LT-INFO", "LT-PROMO", "LT-OTP", "LT-ALERT",
	"MedTest", "MT-INFO", "MT-OTP",
	"LowTest", "LT-API",
	"12345", "67890", "SMS", "API", "TEST",
}

// Шаблоны текстов SMS разной длины и кодировки
var smsTexts = []string{
	// Короткие (1 сегмент GSM7, <160 символов)
	"Ваш код: %06d. Никому не сообщайте.",
	"Заказ #%d подтвержден. Ожидайте доставку.",
	"Баланс: %d.%02d RUB. Спасибо за покупку!",
	"Напоминание: встреча через 30 минут.",
	"Добро пожаловать! Ваш аккаунт активирован.",

	// Средние (1-2 сегмента)
	"Уважаемый клиент! Ваш заказ №%d отправлен. Трек-номер: LT%010d. Ожидаемая дата доставки: завтра.",
	"Акция! Скидка %d%% на все товары до конца недели. Промокод: LOAD%04d. Подробности на сайте.",
	"Ваш платеж на сумму %d.%02d руб. успешно обработан. Номер транзакции: TX-%08d.",

	// Длинные (2-3 сегмента)
	"Информация о рейсе: вылет из SVO в %02d:%02d, прибытие в LED в %02d:%02d. Гейт: %dB. Регистрация открыта. Приятного полета! Контакты: +7-800-555-%04d.",

	// UCS2 (кириллица занимает больше)
	"Привет! Это тестовое сообщение для нагрузочного тестирования SMS-шлюза. Номер теста: %d, время: %s.",
}

// API-ключи клиентов из seed.sql
var clientAPIKeys = []string{
	"lt-high-volume-key-001",
	"lt-med-volume-key-002",
	"lt-low-volume-key-003",
	"test-api-key",
}

// ============================================================
// Генераторы
// ============================================================

var requestCounter uint64

// RandomDestination возвращает случайный номер телефона получателя
func RandomDestination() string {
	prefix := mobileDestPrefixes[rand.Intn(len(mobileDestPrefixes))]
	suffix := rand.Intn(10000000)
	return fmt.Sprintf("%s%07d", prefix, suffix)
}

// RandomInternationalDestination возвращает международный номер
func RandomInternationalDestination() string {
	prefix := intlDestPrefixes[rand.Intn(len(intlDestPrefixes))]
	suffix := rand.Intn(10000000)
	return fmt.Sprintf("%s%07d", prefix, suffix)
}

// RandomSource возвращает случайный source-адрес
func RandomSource() string {
	return sourceAddresses[rand.Intn(len(sourceAddresses))]
}

// RandomText возвращает случайный текст SMS с подставленными данными
func RandomText() string {
	idx := rand.Intn(len(smsTexts))
	template := smsTexts[idx]
	counter := atomic.AddUint64(&requestCounter, 1)

	switch idx {
	case 0:
		return fmt.Sprintf(template, rand.Intn(1000000))
	case 1:
		return fmt.Sprintf(template, counter)
	case 2:
		return fmt.Sprintf(template, rand.Intn(100000), rand.Intn(100))
	case 3, 4:
		return template
	case 5:
		return fmt.Sprintf(template, counter, counter*7)
	case 6:
		return fmt.Sprintf(template, 10+rand.Intn(80), rand.Intn(10000))
	case 7:
		return fmt.Sprintf(template, rand.Intn(100000), rand.Intn(100), counter)
	case 8:
		return fmt.Sprintf(template, rand.Intn(24), rand.Intn(60), rand.Intn(24), rand.Intn(60), rand.Intn(30)+1, rand.Intn(10000))
	case 9:
		return fmt.Sprintf(template, counter, time.Now().Format("15:04:05"))
	default:
		return fmt.Sprintf(template, counter)
	}
}

// RandomAPIKey возвращает случайный API-ключ из пула
func RandomAPIKey() string {
	return clientAPIKeys[rand.Intn(len(clientAPIKeys))]
}

// ============================================================
// Структуры запросов
// ============================================================

// SendSMSPayload генерирует JSON-тело для POST /api/v1/sms/send
type SendSMSPayload struct {
	Source             string `json:"source"`
	Destination        string `json:"destination"`
	Text               string `json:"text"`
	ExternalID         string `json:"external_id,omitempty"`
	Priority           int    `json:"priority,omitempty"`
	RegisteredDelivery bool   `json:"registered_delivery,omitempty"`
}

// NewRandomSendPayload создаёт случайный запрос на отправку SMS
func NewRandomSendPayload() *SendSMSPayload {
	counter := atomic.AddUint64(&requestCounter, 1)
	return &SendSMSPayload{
		Source:             RandomSource(),
		Destination:        RandomDestination(),
		Text:               RandomText(),
		ExternalID:         fmt.Sprintf("lt-%d-%d", time.Now().UnixNano(), counter),
		Priority:           rand.Intn(4), // 0-3
		RegisteredDelivery: rand.Intn(2) == 1,
	}
}

// MarshalJSON сериализует payload в JSON
func (p *SendSMSPayload) JSON() []byte {
	b, _ := json.Marshal(p)
	return b
}

// BatchSMSPayload генерирует тело для POST /api/v1/sms/batch
type BatchSMSPayload struct {
	Messages []SendSMSPayload `json:"messages"`
}

// NewRandomBatchPayload создаёт batch-запрос с n сообщениями
func NewRandomBatchPayload(n int) *BatchSMSPayload {
	msgs := make([]SendSMSPayload, n)
	for i := range msgs {
		msgs[i] = *NewRandomSendPayload()
	}
	return &BatchSMSPayload{Messages: msgs}
}

// JSON сериализует batch payload
func (p *BatchSMSPayload) JSON() []byte {
	b, _ := json.Marshal(p)
	return b
}

// ============================================================
// Профили нагрузки
// ============================================================

// LoadProfile описывает параметры нагрузочного теста
type LoadProfile struct {
	Name              string
	Concurrency       int           // Количество параллельных воркеров
	RequestsPerWorker int           // Запросов на воркера (0 = unlimited, duration-based)
	Duration          time.Duration // Длительность (используется если RequestsPerWorker == 0)
	RampUpDuration    time.Duration // Время нарастания до полной нагрузки
	// Распределение типов запросов (в процентах, сумма = 100)
	SendSMSPercent    int // POST /api/v1/sms/send
	BatchSMSPercent   int // POST /api/v1/sms/batch
	GetStatusPercent  int // GET /api/v1/sms/status/{id}
	GetHistoryPercent int // GET /api/v1/sms/history
	HealthPercent     int // GET /health
}

// Предопределённые профили
var (
	ProfileSmoke = LoadProfile{
		Name:              "smoke",
		Concurrency:       5,
		RequestsPerWorker: 20,
		SendSMSPercent:    80,
		BatchSMSPercent:   10,
		GetStatusPercent:  5,
		GetHistoryPercent: 3,
		HealthPercent:     2,
	}

	ProfileNormal = LoadProfile{
		Name:              "normal",
		Concurrency:       20,
		RequestsPerWorker: 100,
		SendSMSPercent:    70,
		BatchSMSPercent:   15,
		GetStatusPercent:  8,
		GetHistoryPercent: 5,
		HealthPercent:     2,
	}

	ProfileHigh = LoadProfile{
		Name:            "high",
		Concurrency:     100,
		Duration:        2 * time.Minute,
		RampUpDuration:  30 * time.Second,
		SendSMSPercent:  75,
		BatchSMSPercent: 15,
		GetStatusPercent:  5,
		GetHistoryPercent: 3,
		HealthPercent:     2,
	}

	ProfilePeak = LoadProfile{
		Name:              "peak-10k",
		Concurrency:       200,
		Duration:          5 * time.Minute,
		RampUpDuration:    1 * time.Minute,
		SendSMSPercent:    80,
		BatchSMSPercent:   12,
		GetStatusPercent:  4,
		GetHistoryPercent: 2,
		HealthPercent:     2,
	}
)

// PickEndpoint выбирает тип запроса по распределению профиля
func (p *LoadProfile) PickEndpoint() string {
	roll := rand.Intn(100)
	switch {
	case roll < p.SendSMSPercent:
		return "send"
	case roll < p.SendSMSPercent+p.BatchSMSPercent:
		return "batch"
	case roll < p.SendSMSPercent+p.BatchSMSPercent+p.GetStatusPercent:
		return "status"
	case roll < p.SendSMSPercent+p.BatchSMSPercent+p.GetStatusPercent+p.GetHistoryPercent:
		return "history"
	default:
		return "health"
	}
}
