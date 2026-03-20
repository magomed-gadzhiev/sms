package domain

import "errors"

var (
	ErrRouteNotFound       = errors.New("маршрут не найден")
	ErrEmptyPattern        = errors.New("паттерн не может быть пустым")
	ErrInvalidRegex        = errors.New("некорректное регулярное выражение")
	ErrNoProviders         = errors.New("нет доступных провайдеров")
	ErrNoMatchingRoute     = errors.New("нет подходящего маршрута")
	ErrProviderNotFound    = errors.New("провайдер не найден")
	ErrProviderUnavailable = errors.New("провайдер недоступен")

	ErrCountryNotFound       = errors.New("страна не найдена")
	ErrCountryAlreadyExists  = errors.New("страна с таким ISO-кодом уже существует")
	ErrOperatorNotFound      = errors.New("оператор не найден")
	ErrOperatorAlreadyExists = errors.New("оператор с таким кодом уже существует")
	ErrPrefixNotFound        = errors.New("префикс не найден")
)