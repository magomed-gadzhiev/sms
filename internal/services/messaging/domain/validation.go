package domain

import (
	"fmt"
	"strings"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %s: %s", e.Field, e.Message)
}

// MessageValidator валидирует сообщения
type MessageValidator struct{}

// NewMessageValidator создает новый валидатор сообщений
func NewMessageValidator() *MessageValidator {
	return &MessageValidator{}
}

// Validate проверяет сообщение на корректность
func (v *MessageValidator) Validate(msg *Message) error {
	var errors []*ValidationError

	// Проверка source
	if err := v.validateSource(msg.Source); err != nil {
		errors = append(errors, err)
	}

	// Проверка destination
	if err := v.validateDestination(msg.Destination); err != nil {
		errors = append(errors, err)
	}

	// Проверка text
	if err := v.validateText(msg.Text); err != nil {
		errors = append(errors, err)
	}

	// Проверка priority
	if err := v.validatePriority(msg.PriorityFlag); err != nil {
		errors = append(errors, err)
	}

	// Проверка validity period
	if msg.ValidityPeriod != nil && msg.ValidityPeriod.Before(msg.CreatedAt) {
		errors = append(errors, &ValidationError{
			Field:   "validity_period",
			Message: "validity period cannot be in the past",
		})
	}

	if len(errors) > 0 {
		var messages []string
		for _, err := range errors {
			messages = append(messages, err.Error())
		}
		return fmt.Errorf("validation failed: %s", strings.Join(messages, "; "))
	}

	return nil
}

// validateSource проверяет номер отправителя
func (v *MessageValidator) validateSource(source string) *ValidationError {
	if source == "" {
		return &ValidationError{
			Field:   "source",
			Message: "source is required",
		}
	}

	if len(source) > 20 {
		return &ValidationError{
			Field:   "source",
			Message: "source must be 20 characters or less",
		}
	}

	return nil
}

// validateDestination проверяет номер получателя
func (v *MessageValidator) validateDestination(destination string) *ValidationError {
	if destination == "" {
		return &ValidationError{
			Field:   "destination",
			Message: "destination is required",
		}
	}

	if len(destination) > 20 {
		return &ValidationError{
			Field:   "destination",
			Message: "destination must be 20 characters or less",
		}
	}

	// Проверка на допустимые символы (цифры, +, пробелы, дефисы)
	hasDigit := false
	for _, r := range destination {
		if r >= '0' && r <= '9' {
			hasDigit = true
		} else if r != '+' && r != ' ' && r != '-' && r != '(' && r != ')' {
			return &ValidationError{
				Field:   "destination",
				Message: "destination contains invalid characters",
			}
		}
	}

	if !hasDigit {
		return &ValidationError{
			Field:   "destination",
			Message: "destination must contain at least one digit",
		}
	}

	return nil
}

// validateText проверяет текст сообщения
func (v *MessageValidator) validateText(text string) *ValidationError {
	if text == "" {
		return &ValidationError{
			Field:   "text",
			Message: "text is required",
		}
	}

	if len(text) > 1600 {
		return &ValidationError{
			Field:   "text",
			Message: "text must be 1600 characters or less",
		}
	}

	return nil
}

// validatePriority проверяет приоритет сообщения
func (v *MessageValidator) validatePriority(priority int) *ValidationError {
	if priority < 0 || priority > 3 {
		return &ValidationError{
			Field:   "priority",
			Message: "priority must be between 0 and 3",
		}
	}
	return nil
}

// DetectEncoding определяет кодировку текста сообщения
func DetectEncoding(text string) shared.MessageEncoding {
	// Простая проверка - если все символы в ASCII диапазоне, используем GSM7
	for _, r := range text {
		if r > 127 {
			return shared.MessageEncodingUCS2
		}
	}
	return shared.MessageEncodingGSM7
}
