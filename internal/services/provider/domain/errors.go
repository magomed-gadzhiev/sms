package domain

import "errors"

var (
	ErrProviderNotFound              = errors.New("провайдер не найден")
	ErrProviderNameRequired          = errors.New("название провайдера обязательно")
	ErrProviderHostRequired          = errors.New("хост провайдера обязателен")
	ErrProviderPortInvalid           = errors.New("порт провайдера должен быть в диапазоне 1-65535")
	ErrProviderSystemIDRequired      = errors.New("system_id провайдера обязателен")
	ErrProviderPasswordRequired      = errors.New("пароль провайдера обязателен")
	ErrProviderMaxConnectionsInvalid = errors.New("максимальное количество соединений должно быть >= 0")
	ErrProviderBindTypeInvalid       = errors.New("неверный тип bind (должен быть transceiver, transmitter или receiver)")
	ErrConnectionNotFound            = errors.New("соединение не найдено")
	ErrConnectionNotBound            = errors.New("соединение не привязано")
	ErrConnectionPoolFull            = errors.New("пул соединений заполнен")
	ErrProviderInactive              = errors.New("провайдер неактивен")
)
