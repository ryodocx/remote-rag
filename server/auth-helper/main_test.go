package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/trace/noop"
)

// MockCache is a mock implementation of Cache interface.
type MockCache struct {
	GetFn func(ctx context.Context, key string) (string, error)
	SetFn func(ctx context.Context, key string, value string, ttl time.Duration) error
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, key)
	}
	return "", ErrCacheMiss
}

func (m *MockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if m.SetFn != nil {
		return m.SetFn(ctx, key, value, ttl)
	}
	return nil
}

// MockKeyfunc implements keyfunc.Keyfunc interface.
type MockKeyfunc struct {
	KeyfuncFn func(token *jwt.Token) (interface{}, error)
}

func (m *MockKeyfunc) Keyfunc(token *jwt.Token) (interface{}, error) {
	return m.KeyfuncFn(token)
}

func TestReadyzHandler(t *testing.T) {
	origCache := cacheInstance
	defer func() { cacheInstance = origCache }()

	// Case 1: Cache returns cache miss -> Readyz should be 200 OK
	cacheInstance = &MockCache{
		GetFn: func(ctx context.Context, key string) (string, error) {
			return "", ErrCacheMiss
		},
	}

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	readyzHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "ready" {
		t.Errorf("Expected 'ready', got %q", w.Body.String())
	}

	// Case 2: Cache returns connection error -> Readyz should be 503 Service Unavailable
	cacheInstance = &MockCache{
		GetFn: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("redis connection refused")
		},
	}

	req = httptest.NewRequest("GET", "/readyz", nil)
	w = httptest.NewRecorder()
	readyzHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestHealthzHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	healthzHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("Expected 'ok', got %q", w.Body.String())
	}
}

func TestValidateClientConstraints(t *testing.T) {
	// Backup environment variables
	envVars := []string{
		"AUTH_FILTER_ISS",
		"AUTH_FILTER_AUD",
		"AUTH_FILTER_CLIENT_ID",
		"AUTH_FILTER_SCOPES",
	}
	origEnv := make(map[string]string)
	for _, v := range envVars {
		origEnv[v] = os.Getenv(v)
	}
	defer func() {
		for k, v := range origEnv {
			os.Setenv(k, v)
		}
	}()

	span := noop.NewTracerProvider().Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	// Clear variables for base test
	for _, v := range envVars {
		os.Setenv(v, "")
	}

	res := &IntrospectionResponse{
		Active:   true,
		Iss:      "https://issuer.com",
		Aud:      "my-audience",
		ClientID: "client-123",
		Scope:    "read write openid",
	}

	// No filters set -> should pass
	if !validateClientConstraints(res, span) {
		t.Errorf("Expected base constraints to pass")
	}

	// Issuer filter
	os.Setenv("AUTH_FILTER_ISS", "https://issuer.com")
	if !validateClientConstraints(res, span) {
		t.Errorf("Expected matching issuer to pass")
	}
	os.Setenv("AUTH_FILTER_ISS", "https://wrong.com")
	if validateClientConstraints(res, span) {
		t.Errorf("Expected wrong issuer to fail")
	}
	os.Setenv("AUTH_FILTER_ISS", "")

	// Audience filter (single string)
	os.Setenv("AUTH_FILTER_AUD", "my-audience")
	if !validateClientConstraints(res, span) {
		t.Errorf("Expected matching audience to pass")
	}
	os.Setenv("AUTH_FILTER_AUD", "wrong-aud")
	if validateClientConstraints(res, span) {
		t.Errorf("Expected wrong audience to fail")
	}

	// Audience filter (array of strings)
	res.Aud = []interface{}{"another-aud", "my-audience"}
	os.Setenv("AUTH_FILTER_AUD", "my-audience")
	if !validateClientConstraints(res, span) {
		t.Errorf("Expected matching audience in slice to pass")
	}
	os.Setenv("AUTH_FILTER_AUD", "not-present")
	if validateClientConstraints(res, span) {
		t.Errorf("Expected wrong audience in slice to fail")
	}
	os.Setenv("AUTH_FILTER_AUD", "")

	// ClientID filter
	os.Setenv("AUTH_FILTER_CLIENT_ID", "client-123")
	if !validateClientConstraints(res, span) {
		t.Errorf("Expected matching client ID to pass")
	}
	os.Setenv("AUTH_FILTER_CLIENT_ID", "wrong-client")
	if validateClientConstraints(res, span) {
		t.Errorf("Expected wrong client ID to fail")
	}
	os.Setenv("AUTH_FILTER_CLIENT_ID", "")

	// Scope filter
	os.Setenv("AUTH_FILTER_SCOPES", "write")
	if !validateClientConstraints(res, span) {
		t.Errorf("Expected matching scope to pass")
	}
	os.Setenv("AUTH_FILTER_SCOPES", "admin")
	if validateClientConstraints(res, span) {
		t.Errorf("Expected wrong scope to fail")
	}
}

func TestValidateUserConstraints(t *testing.T) {
	// Backup environment variables
	envVars := []string{
		"AUTH_FILTER_EMAIL_DOMAINS",
		"AUTH_FILTER_EMAILS",
		"AUTH_FILTER_GROUPS",
		"AUTH_FILTER_SUBJECTS",
	}
	origEnv := make(map[string]string)
	for _, v := range envVars {
		origEnv[v] = os.Getenv(v)
	}
	defer func() {
		for k, v := range origEnv {
			os.Setenv(k, v)
		}
	}()

	span := noop.NewTracerProvider().Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	// Clear variables for base test
	for _, v := range envVars {
		os.Setenv(v, "")
	}

	res := &IntrospectionResponse{
		Active: true,
		Sub:    "user-999",
		Email:  "john.doe@company.com",
		Groups: []interface{}{"developers", "users"},
	}

	// No filters set -> should pass
	if !validateUserConstraints(res, span) {
		t.Errorf("Expected base user constraints to pass")
	}

	// Email domain filter
	os.Setenv("AUTH_FILTER_EMAIL_DOMAINS", "company.com,partner.org")
	if !validateUserConstraints(res, span) {
		t.Errorf("Expected matching email domain to pass")
	}
	os.Setenv("AUTH_FILTER_EMAIL_DOMAINS", "gmail.com")
	if validateUserConstraints(res, span) {
		t.Errorf("Expected wrong email domain to fail")
	}
	os.Setenv("AUTH_FILTER_EMAIL_DOMAINS", "")

	// Email filter
	os.Setenv("AUTH_FILTER_EMAILS", "john.doe@company.com, admin@company.com")
	if !validateUserConstraints(res, span) {
		t.Errorf("Expected matching email to pass")
	}
	os.Setenv("AUTH_FILTER_EMAILS", "someone.else@company.com")
	if validateUserConstraints(res, span) {
		t.Errorf("Expected wrong email to fail")
	}
	os.Setenv("AUTH_FILTER_EMAILS", "")

	// Groups filter
	os.Setenv("AUTH_FILTER_GROUPS", "developers,admins")
	if !validateUserConstraints(res, span) {
		t.Errorf("Expected matching group in slice to pass")
	}
	os.Setenv("AUTH_FILTER_GROUPS", "managers")
	if validateUserConstraints(res, span) {
		t.Errorf("Expected wrong group to fail")
	}

	// Groups filter (single string type)
	res.Groups = "users"
	os.Setenv("AUTH_FILTER_GROUPS", "users,guests")
	if !validateUserConstraints(res, span) {
		t.Errorf("Expected matching group string to pass")
	}
	os.Setenv("AUTH_FILTER_GROUPS", "admins")
	if validateUserConstraints(res, span) {
		t.Errorf("Expected wrong group string to fail")
	}
	os.Setenv("AUTH_FILTER_GROUPS", "")

	// Subject filter
	os.Setenv("AUTH_FILTER_SUBJECTS", "user-999,user-888")
	if !validateUserConstraints(res, span) {
		t.Errorf("Expected matching subject to pass")
	}
	os.Setenv("AUTH_FILTER_SUBJECTS", "user-111")
	if validateUserConstraints(res, span) {
		t.Errorf("Expected wrong subject to fail")
	}
}

func TestIntrospectToken_HTTPCall(t *testing.T) {
	// Setup mock OAuth IDP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected form urlencoded, got %q", r.Header.Get("Content-Type"))
		}

		// Check Basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "client-id" || password != "client-secret" {
			t.Errorf("Invalid Basic auth credentials")
		}

		err := r.ParseForm()
		if err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}
		token := r.Form.Get("token")

		w.Header().Set("Content-Type", "application/json")
		if token == "valid-token" {
			fmt.Fprint(w, `{"active": true, "sub": "test-user"}`)
		} else {
			fmt.Fprint(w, `{"active": false}`)
		}
	}))
	defer server.Close()

	// Backup variables
	origURL := oauthIntrospectURL
	origClientID := oauthClientID
	origClientSecret := oauthClientSecret
	origClient := httpClient

	defer func() {
		oauthIntrospectURL = origURL
		oauthClientID = origClientID
		oauthClientSecret = origClientSecret
		httpClient = origClient
	}()

	oauthIntrospectURL = server.URL
	oauthClientID = "client-id"
	oauthClientSecret = "client-secret"
	httpClient = &http.Client{Timeout: 5 * time.Second}

	// Check active token
	res, err := introspectToken(context.Background(), "valid-token")
	if err != nil {
		t.Fatalf("introspectToken failed: %v", err)
	}
	if !res.Active {
		t.Errorf("Expected active to be true")
	}
	if res.Sub != "test-user" {
		t.Errorf("Expected sub to be 'test-user', got %s", res.Sub)
	}

	// Check inactive token
	res, err = introspectToken(context.Background(), "invalid-token")
	if err != nil {
		t.Fatalf("introspectToken failed: %v", err)
	}
	if res.Active {
		t.Errorf("Expected active to be false")
	}
}

func TestIntrospectHandler_Flow(t *testing.T) {
	// Setup Introspection Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"active": true, "sub": "john", "client_id": "test-client"}`)
	}))
	defer server.Close()

	// Backup globals
	origCache := cacheInstance
	origURL := oauthIntrospectURL
	origClientID := oauthClientID
	origClientSecret := oauthClientSecret
	origTTL := introspectCacheTTL
	origClient := httpClient

	defer func() {
		cacheInstance = origCache
		oauthIntrospectURL = origURL
		oauthClientID = origClientID
		oauthClientSecret = origClientSecret
		introspectCacheTTL = origTTL
		httpClient = origClient
	}()

	cacheInstance = NewMemoryCache()
	oauthIntrospectURL = server.URL
	oauthClientID = "my-client"
	oauthClientSecret = "my-secret"
	introspectCacheTTL = 10 * time.Minute
	httpClient = &http.Client{Timeout: 5 * time.Second}

	// Make request without auth header -> should be 410 / StatusUnauthorized
	req := httptest.NewRequest("GET", "/auth/introspect", nil)
	w := httptest.NewRecorder()
	introspectHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}

	// Make request with valid auth header
	req = httptest.NewRequest("GET", "/auth/introspect", nil)
	req.Header.Set("Authorization", "Bearer val-token-abc")
	w = httptest.NewRecorder()
	introspectHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	// Check if token was cached. If we query again, it should hit cache.
	// We verify by changing oauthIntrospectURL to invalid URL.
	// If it doesn't hit cache, it will try to call the bad URL and fail/error out.
	oauthIntrospectURL = "http://invalid-localhost-url/introspect"
	
	req = httptest.NewRequest("GET", "/auth/introspect", nil)
	req.Header.Set("Authorization", "Bearer val-token-abc")
	w = httptest.NewRecorder()
	introspectHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected cached 200 OK, got %d", w.Code)
	}
}

func TestJWKSHandler_Flow(t *testing.T) {
	origJWKS := jwksInstance
	defer func() { jwksInstance = origJWKS }()

	myKey := []byte("secret-test-key-hmac")
	jwksInstance = &MockKeyfunc{
		KeyfuncFn: func(token *jwt.Token) (interface{}, error) {
			return myKey, nil
		},
	}

	// Sign a token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":       "test-issuer",
		"aud":       "test-audience",
		"client_id": "test-client",
		"scope":     "openid",
		"sub":       "user-555",
		"email":     "user555@company.com",
	})
	tokenString, err := token.SignedString(myKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	// Call handler with valid expected_aud
	req := httptest.NewRequest("GET", "/auth/jwks?expected_aud=test-audience", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	jwksHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	// Call handler with missing expected_aud query param -> 400 Bad Request
	req = httptest.NewRequest("GET", "/auth/jwks", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w = httptest.NewRecorder()
	jwksHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", w.Code)
	}

	// Call handler with mismatching expected_aud -> 401 Unauthorized
	req = httptest.NewRequest("GET", "/auth/jwks?expected_aud=wrong-audience", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w = httptest.NewRecorder()
	jwksHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}
