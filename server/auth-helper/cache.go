package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache interface abstracts the underlying caching mechanism.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

var ErrCacheMiss = errors.New("cache miss")

// ---------------------------------------------------------
// RedisCache Implementation
// ---------------------------------------------------------
type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(host, port string) *RedisCache {
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "6379"
	}
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port),
	})
	return &RedisCache{client: client}
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

func (r *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// ---------------------------------------------------------
// MemoryCache Implementation
// ---------------------------------------------------------
type memoryItem struct {
	value  string
	expiry time.Time
}

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]memoryItem),
	}
}

func (m *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, found := m.items[key]
	if !found {
		return "", ErrCacheMiss
	}
	if time.Now().After(item.expiry) {
		return "", ErrCacheMiss
	}
	return item.value, nil
}

func (m *MemoryCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = memoryItem{
		value:  value,
		expiry: time.Now().Add(ttl),
	}
	return nil
}
