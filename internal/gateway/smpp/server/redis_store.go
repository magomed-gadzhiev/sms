package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrRedisKeyNotFound = errors.New("redis key not found")

type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

type MessageMapping struct {
	SystemID   string    `json:"system_id"`
	SourceAddr string    `json:"source_addr"`
	DestAddr   string    `json:"dest_addr"`
	SubmitDate time.Time `json:"submit_date"`
}

type SessionBinding struct {
	GatewayAddr string `json:"gateway_addr"`
}

type RedisStore struct {
	client     RedisClient
	messageTTL time.Duration
	sessionTTL time.Duration
}

func NewRedisStore(client RedisClient, messageTTL, sessionTTL time.Duration) *RedisStore {
	return &RedisStore{client: client, messageTTL: messageTTL, sessionTTL: sessionTTL}
}

func (s *RedisStore) SaveMessageMapping(ctx context.Context, messageID string, mapping *MessageMapping) error {
	data, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("ошибка сериализации message mapping: %w", err)
	}
	return s.client.Set(ctx, fmt.Sprintf("smpp:msg:%s", messageID), string(data), s.messageTTL)
}

func (s *RedisStore) GetMessageMapping(ctx context.Context, messageID string) (*MessageMapping, error) {
	val, err := s.client.Get(ctx, fmt.Sprintf("smpp:msg:%s", messageID))
	if err != nil {
		return nil, err
	}
	var mapping MessageMapping
	if err := json.Unmarshal([]byte(val), &mapping); err != nil {
		return nil, fmt.Errorf("ошибка десериализации message mapping: %w", err)
	}
	return &mapping, nil
}

func (s *RedisStore) SaveSessionBinding(ctx context.Context, systemID string, binding *SessionBinding) error {
	data, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("ошибка сериализации session binding: %w", err)
	}
	return s.client.Set(ctx, fmt.Sprintf("smpp:session:%s", systemID), string(data), s.sessionTTL)
}

func (s *RedisStore) GetSessionBinding(ctx context.Context, systemID string) (*SessionBinding, error) {
	val, err := s.client.Get(ctx, fmt.Sprintf("smpp:session:%s", systemID))
	if err != nil {
		return nil, err
	}
	var binding SessionBinding
	if err := json.Unmarshal([]byte(val), &binding); err != nil {
		return nil, fmt.Errorf("ошибка десериализации session binding: %w", err)
	}
	return &binding, nil
}

func (s *RedisStore) DeleteSessionBinding(ctx context.Context, systemID string) error {
	return s.client.Del(ctx, fmt.Sprintf("smpp:session:%s", systemID))
}

func (s *RedisStore) RefreshSessionTTL(ctx context.Context, systemID string) error {
	return s.client.Expire(ctx, fmt.Sprintf("smpp:session:%s", systemID), s.sessionTTL)
}
