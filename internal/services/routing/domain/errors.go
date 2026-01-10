package domain

import "errors"

var (
	ErrRouteNotFound     = errors.New("маршрут не найден")
	ErrEmptyPattern      = errors.New("паттерн не может быть пустым")
	ErrInvalidRegex      = errors.New("некорректное регулярное выражение")
	ErrNoProviders       = errors.New("нет доступных провайдеров")
	ErrNoMatchingRoute   = errors.New("нет подходящего маршрута")
	ErrProviderNotFound  = errors.New("провайдер не найден")
	ErrProviderUnavailable = errors.New("провайдер недоступен")
)