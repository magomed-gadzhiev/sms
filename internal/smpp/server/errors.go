package server

import "errors"

var (
	// ErrSessionNotFound ошибка, когда сессия не найдена
	ErrSessionNotFound = errors.New("сессия не найдена")
	
	// ErrSessionNotBound ошибка, когда сессия не привязана
	ErrSessionNotBound = errors.New("сессия не привязана")
	
	// ErrInvalidBindState ошибка неверного состояния для bind
	ErrInvalidBindState = errors.New("неверное состояние для bind операции")
	
	// ErrRateLimitExceeded ошибка превышения rate limit
	ErrRateLimitExceeded = errors.New("превышен rate limit")
	
	// ErrConnectionClosed ошибка закрытого соединения
	ErrConnectionClosed = errors.New("соединение закрыто")
	
	// ErrInvalidPDU ошибка неверного PDU
	ErrInvalidPDU = errors.New("неверный PDU")
	
	// ErrReadTimeout ошибка таймаута чтения
	ErrReadTimeout = errors.New("таймаут чтения")
	
	// ErrWriteTimeout ошибка таймаута записи
	ErrWriteTimeout = errors.New("таймаут записи")
)
