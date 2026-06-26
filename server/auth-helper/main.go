package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient      *redis.Client
	oauthIntrospectURL string
	oauthClientID      string
	oauthClientSecret  string
	ctx              = context.Background()
)

func init() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

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

	// Check Redis cache
	val, err := redisClient.Get(ctx, tokenHash).Result()
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
		redisClient.Set(ctx, tokenHash, "valid", 60*time.Second)
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
