package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTransmitter_StartAndSend(t *testing.T) {
	// Setup mock POST target server
	var mu sync.Mutex
	receivedMsgs := []string{}
	receivedHeaders := []http.Header{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read POST body: %v", err)
		}

		mu.Lock()
		receivedMsgs = append(receivedMsgs, string(bodyBytes))
		receivedHeaders = append(receivedHeaders, r.Header)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Backup stdin, keyring
	origStdin := stdin
	origGet := keyringGet
	defer func() {
		stdin = origStdin
		keyringGet = origGet
	}()

	// Mock stdin input
	stdin = strings.NewReader("message1\nmessage2\n")

	// Mock keyring token
	mockToken := TokenData{
		AccessToken: "transmitter-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	tokenBytes, _ := json.Marshal(mockToken)
	keyringGet = func(service, user string) (string, error) {
		return string(tokenBytes), nil
	}

	postURLChan := make(chan string, 1)
	postURLChan <- server.URL

	transmitter := NewTransmitter(&Config{UseIDToken: false}, postURLChan)

	// Run transmitter Start which reads from mocked stdin and sends to server.
	// Since stdin has two lines and then hits EOF, it will close StdinChan and return.
	transmitter.Start()

	mu.Lock()
	defer mu.Unlock()

	if len(receivedMsgs) != 2 {
		t.Fatalf("Expected 2 received messages, got %d", len(receivedMsgs))
	}

	if receivedMsgs[0] != "message1" {
		t.Errorf("Expected message1, got %s", receivedMsgs[0])
	}
	if receivedMsgs[1] != "message2" {
		t.Errorf("Expected message2, got %s", receivedMsgs[1])
	}

	for i, h := range receivedHeaders {
		auth := h.Get("Authorization")
		if auth != "Bearer transmitter-token" {
			t.Errorf("Msg %d: Expected Authorization 'Bearer transmitter-token', got %q", i+1, auth)
		}
		ct := h.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Msg %d: Expected Content-Type 'application/json', got %q", i+1, ct)
		}
	}
}
