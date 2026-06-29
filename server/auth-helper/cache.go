package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache interface はバックエンドのキャッシュ機構（Redis、メモリ等）を抽象化します。
type Cache interface {
	// Get は指定したキーに対応する値をキャッシュから取得します。存在しない場合は ErrCacheMiss を返します。
	Get(ctx context.Context, key string) (string, error)
	// Set は指定したキーと値のペアをTTL（有効期限）付きでキャッシュに保存します。
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

var ErrCacheMiss = errors.New("cache miss")

// ---------------------------------------------------------
// RedisCache 実装
// ---------------------------------------------------------

// RedisCache は go-redis を用いて外部の Redis サーバーに接続するキャッシュ実装です。
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
// MemoryCache 実装
// ---------------------------------------------------------

// memoryItem はインメモリキャッシュに保存する値と有効期限を保持する構造体です。
type memoryItem struct {
	value  string
	expiry time.Time
}

// MemoryCache は Go の組み込みマップと RWMutex を利用した、スレッドセーフなインメモリキャッシュ実装です。
// ローカルでのテストや、Redisコンテナを起動したくない軽量な環境での利用を想定しています。
type MemoryCache struct {
	mu     sync.RWMutex
	items  map[string]memoryItem
	stopCh chan struct{}
}

func NewMemoryCache() *MemoryCache {
	m := &MemoryCache{
		items:  make(map[string]memoryItem),
		stopCh: make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

func (m *MemoryCache) Close() error {
	close(m.stopCh)
	return nil
}

func (m *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for k, v := range m.items {
				if now.After(v.expiry) {
					delete(m.items, k)
				}
			}
			m.mu.Unlock()
		case <-m.stopCh:
			return
		}
	}
}

func (m *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	item, found := m.items[key]
	m.mu.RUnlock()
	
	if !found {
		return "", ErrCacheMiss
	}
	
	if time.Now().After(item.expiry) {
		// ダブルチェック: Lockを取得してから再度期限切れと存在を確認する
		m.mu.Lock()
		defer m.mu.Unlock()
		
		item, found = m.items[key]
		if found && time.Now().After(item.expiry) {
			delete(m.items, key)
		}
		
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
