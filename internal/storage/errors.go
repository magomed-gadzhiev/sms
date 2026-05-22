package storage

import "errors"

// ErrNotFound возвращается, когда запись не найдена в базе данных
var ErrNotFound = errors.New("record not found")

// ErrDuplicateKey возвращается при нарушении уникального ключа
var ErrDuplicateKey = errors.New("duplicate key violation")
