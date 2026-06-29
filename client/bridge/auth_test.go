package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGetAccountName(t *testing.T) {
	// Backup env
	origProfile := os.Getenv("RRAG_PROFILE")
	origClientID := os.Getenv("OAUTH_CLIENT_ID")
	defer func() {
		os.Setenv("RRAG_PROFILE", origProfile)
		os.Setenv("OAUTH_CLIENT_ID", origClientID)
	}()

	os.Setenv("RRAG_PROFILE", "test-profile")
	if getAccountName() != "oauth-token-test-profile" {
		t.Errorf("Expected oauth-token-test-profile, got %s", getAccountName())
	}

	os.Setenv("RRAG_PROFILE", "")
	os.Setenv("OAUTH_CLIENT_ID", "test-client")
	if getAccountName() != "oauth-token-test-client" {
		t.Errorf("Expected oauth-token-test-client, got %s", getAccountName())
	}

	os.Setenv("OAUTH_CLIENT_ID", "")
	if getAccountName() != "oauth-token" {
		t.Errorf("Expected oauth-token, got %s", getAccountName())
	}
}

func TestGeneratePKCE(t *testing.T) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE failed: %v", err)
	}
	if len(verifier) == 0 || len(challenge) == 0 {
		t.Errorf("Empty verifier or challenge")
	}
}

func TestGetValidToken_KeyringHit(t *testing.T) {
	// Setup mocks
	originalGet := keyringGet
	defer func() { keyringGet = originalGet }()

	expectedToken := TokenData{
		AccessToken: "mock-access-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	tokenBytes, _ := json.Marshal(expectedToken)

	keyringGet = func(service, user string) (string, error) {
		if service == serviceName && user == getAccountName() {
			return string(tokenBytes), nil
		}
		return "", fmt.Errorf("not found")
	}

	token, err := GetValidToken()
	if err != nil {
		t.Fatalf("GetValidToken failed: %v", err)
	}

	if token.AccessToken != expectedToken.AccessToken {
		t.Errorf("Expected access token %s, got %s", expectedToken.AccessToken, token.AccessToken)
	}
}

func TestAuthenticateAndGetValidToken_Refresh(t *testing.T) {
	// Setup Token Refresh Server
	var refreshCallCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		refreshCallCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		// Return new token
		fmt.Fprint(w, `{"access_token": "new-refreshed-token", "refresh_token": "still-refresh", "expires_in": 3600}`)
	}))
	defer server.Close()

	// Backup keyring and env
	origGet := keyringGet
	origSet := keyringSet
	origOpen := openBrowserFn
	origClientID := os.Getenv("OAUTH_CLIENT_ID")
	origAuthURL := os.Getenv("OAUTH_AUTH_URL")
	origTokenURL := os.Getenv("OAUTH_TOKEN_URL")
	origPort := os.Getenv("OAUTH_REDIRECT_PORT")

	defer func() {
		keyringGet = origGet
		keyringSet = origSet
		openBrowserFn = origOpen
		os.Setenv("OAUTH_CLIENT_ID", origClientID)
		os.Setenv("OAUTH_AUTH_URL", origAuthURL)
		os.Setenv("OAUTH_TOKEN_URL", origTokenURL)
		os.Setenv("OAUTH_REDIRECT_PORT", origPort)
	}()

	os.Setenv("OAUTH_CLIENT_ID", "test-client")
	os.Setenv("OAUTH_AUTH_URL", server.URL+"/auth")
	os.Setenv("OAUTH_TOKEN_URL", server.URL+"/token")
	os.Setenv("OAUTH_REDIRECT_PORT", "18085")

	// Store expired token
	expiredToken := TokenData{
		AccessToken:  "old-expired-token",
		RefreshToken: "refresh-token-val",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}
	expiredBytes, _ := json.Marshal(expiredToken)

	var savedData TokenData
	keyringGet = func(service, user string) (string, error) {
		return string(expiredBytes), nil
	}
	keyringSet = func(service, user, secret string) error {
		json.Unmarshal([]byte(secret), &savedData)
		return nil
	}

	token, err := GetValidToken()
	if err != nil {
		t.Fatalf("GetValidToken failed: %v", err)
	}

	if token.AccessToken != "new-refreshed-token" {
		t.Errorf("Expected new-refreshed-token, got %s", token.AccessToken)
	}

	mu.Lock()
	if refreshCallCount != 1 {
		t.Errorf("Expected 1 call to refresh server, got %d", refreshCallCount)
	}
	mu.Unlock()

	if savedData.AccessToken != "new-refreshed-token" {
		t.Errorf("Expected saved access token to be updated, got %s", savedData.AccessToken)
	}
}

func TestAuthenticate_FullFlow(t *testing.T) {
	// Mock IDP and Token Exchange Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token": "auth-flow-access-token", "id_token": "auth-flow-id-token", "expires_in": 3600}`)
	}))
	defer server.Close()

	// Backup keyring and env
	origGet := keyringGet
	origSet := keyringSet
	origOpen := openBrowserFn
	origClientID := os.Getenv("OAUTH_CLIENT_ID")
	origAuthURL := os.Getenv("OAUTH_AUTH_URL")
	origTokenURL := os.Getenv("OAUTH_TOKEN_URL")
	origPort := os.Getenv("OAUTH_REDIRECT_PORT")

	defer func() {
		keyringGet = origGet
		keyringSet = origSet
		openBrowserFn = origOpen
		os.Setenv("OAUTH_CLIENT_ID", origClientID)
		os.Setenv("OAUTH_AUTH_URL", origAuthURL)
		os.Setenv("OAUTH_TOKEN_URL", origTokenURL)
		os.Setenv("OAUTH_REDIRECT_PORT", origPort)
	}()

	os.Setenv("OAUTH_CLIENT_ID", "test-client")
	os.Setenv("OAUTH_AUTH_URL", server.URL+"/auth")
	os.Setenv("OAUTH_TOKEN_URL", server.URL+"/token")
	// Use port 18086 for this test to avoid collision
	os.Setenv("OAUTH_REDIRECT_PORT", "18086")

	keyringGet = func(service, user string) (string, error) {
		return "", fmt.Errorf("token not found")
	}

	var savedData TokenData
	keyringSet = func(service, user, secret string) error {
		json.Unmarshal([]byte(secret), &savedData)
		return nil
	}

	// Mock openBrowserFn to simulate callback
	openBrowserFn = func(authURLStr string) error {
		u, err := url.Parse(authURLStr)
		if err != nil {
			return err
		}
		state := u.Query().Get("state")
		
		// Send callback request in goroutine
		go func() {
			time.Sleep(10 * time.Millisecond)
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:18086/callback?state=%s&code=mock_code", state))
			if err != nil {
				t.Errorf("Failed to make callback request: %v", err)
				return
			}
			resp.Body.Close()
		}()
		return nil
	}

	token, err := Authenticate()
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	if token.AccessToken != "auth-flow-access-token" {
		t.Errorf("Expected auth-flow-access-token, got %s", token.AccessToken)
	}

	if savedData.AccessToken != "auth-flow-access-token" {
		t.Errorf("Expected saved token to match, got %s", savedData.AccessToken)
	}
}
