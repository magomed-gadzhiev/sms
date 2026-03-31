package max_messenger

// SendRequest — тело запроса к Max Bot API для отправки сообщения
type SendRequest struct {
	RecipientMSISDN string          `json:"recipient"`
	Text            string          `json:"text,omitempty"`
	ImageURL        string          `json:"image_url,omitempty"`
	InlineKeyboard  [][]ButtonItem  `json:"inline_keyboard,omitempty"`
	CallbackURL     string          `json:"callback_url"`
	ExternalID      string          `json:"external_id"`
}

// SendResponse — ответ Max Bot API на отправку сообщения
type SendResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// ButtonItem — элемент inline-кнопки
type ButtonItem struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
	Data string `json:"callback_data,omitempty"`
}

// CheckRegistrationRequest — запрос проверки регистрации MSISDN в Max
type CheckRegistrationRequest struct {
	MSISDN string `json:"msisdn"`
}

// CheckRegistrationResponse — ответ проверки регистрации
type CheckRegistrationResponse struct {
	Registered bool   `json:"registered"`
	UserID     string `json:"user_id,omitempty"`
}

// WebhookPayload — входящий webhook от Max API со статусом доставки
type WebhookPayload struct {
	AttemptID string `json:"attempt_id"`
	MessageID string `json:"message_id"`
	Status    string `json:"status"` // delivered | read | error
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}
