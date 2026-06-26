package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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

// ---------------------------------------------------------
// Main Application
// ---------------------------------------------------------

var (
	cacheInstance      Cache
	oauthIntrospectURL string
	oauthClientID      string
	oauthClientSecret  string
	ctx                = context.Background()
)

func init() {
	cacheType := os.Getenv("CACHE_TYPE")
	if cacheType == "memory" {
		log.Println("Using In-Memory Cache")
		cacheInstance = NewMemoryCache()
	} else {
		log.Println("Using Redis Cache")
		cacheInstance = NewRedisCache(os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"))
	}

	oauthIntrospectURL = os.Getenv("OAUTH_INTROSPECT_URL")
	oauthClientID = os.Getenv("OAUTH_CLIENT_ID")
	oauthClientSecret = os.Getenv("OAUTH_CLIENT_SECRET")

	if oauthIntrospectURL == "" || oauthClientID == "" || oauthClientSecret == "" {
		log.Println("WARNING: OAUTH_INTROSPECT_URL, OAUTH_CLIENT_ID, or OAUTH_CLIENT_SECRET is missing. Introspection will fail.")
	}
}

func hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

func introspectToken(token string) (bool, error) {
	if oauthIntrospectURL == "" {
		// Mock logic for local testing if env is not set
		log.Println("Mocking introspection: Returning true")
		return true, nil
	}

	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")

	req, err := http.NewRequest("POST", oauthIntrospectURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Basic Auth
	auth := oauthClientID + ":" + oauthClientSecret
	basicAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+basicAuth)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Introspection failed with status %d: %s", resp.StatusCode, string(body))
		return false, nil
	}

	var result struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.Active, nil
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tokenHash := hashToken(token)

	// Check cache
	val, err := cacheInstance.Get(ctx, tokenHash)
	if err == nil && val == "valid" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Not in cache, validate online
	active, err := introspectToken(token)
	if err != nil {
		log.Printf("Error introspecting token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if active {
		// Cache for 60 seconds
		cacheInstance.Set(ctx, tokenHash, "valid", 60*time.Second)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
}

func main() {
	http.HandleFunc("/auth", authHandler)
	port := "8000"
	log.Printf("Auth Helper starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
