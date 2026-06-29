package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestSSEClient_GetToken(t *testing.T) {
	origGet := keyringGet
	defer func() { keyringGet = origGet }()

	mockToken := TokenData{
		AccessToken:  "access-123",
		IDToken:      "id-456",
		RefreshToken: "refresh-789",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	tokenBytes, _ := json.Marshal(mockToken)
	keyringGet = func(service, user string) (string, error) {
		return string(tokenBytes), nil
	}

	// Test without ID token
	client1 := NewSSEClient(&Config{UseIDToken: false})
	tok1, err := client1.getToken()
	if err != nil {
		t.Fatalf("getToken failed: %v", err)
	}
	if tok1 != "access-123" {
		t.Errorf("Expected access-123, got %s", tok1)
	}

	// Test with ID token
	client2 := NewSSEClient(&Config{UseIDToken: true})
	tok2, err := client2.getToken()
	if err != nil {
		t.Fatalf("getToken failed: %v", err)
	}
	if tok2 != "id-456" {
		t.Errorf("Expected id-456, got %s", tok2)
	}
}

func TestSSEClient_ReadStream_MessageEvent(t *testing.T) {
	origStdout := stdout
	defer func() { stdout = origStdout }()

	var mockStdout bytes.Buffer
	stdout = &mockStdout

	client := NewSSEClient(&Config{RemoteURL: "http://example.com/mcp/sse"})

	streamContent := "event: message\ndata: {\"mcp\":\"event\"}\n\n"
	body := bytes.NewReader([]byte(streamContent))

	client.readStream(body)

	expected := "{\"mcp\":\"event\"}\n"
	if mockStdout.String() != expected {
		t.Errorf("Expected stdout to be %q, got %q", expected, mockStdout.String())
	}
}

func TestSSEClient_ReadStream_EndpointEvent(t *testing.T) {
	client := NewSSEClient(&Config{RemoteURL: "http://example.com/mcp/sse"})

	// Absolute endpoint URI
	streamContent1 := "event: endpoint\ndata: http://localhost:9000/messages\n\n"
	client.readStream(bytes.NewReader([]byte(streamContent1)))

	select {
	case url := <-client.PostURLChan:
		if url != "http://localhost:9000/messages" {
			t.Errorf("Expected http://localhost:9000/messages, got %s", url)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Timed out waiting for endpoint url")
	}

	// Relative endpoint URI
	streamContent2 := "event: endpoint\ndata: /messages\n\n"
	client.readStream(bytes.NewReader([]byte(streamContent2)))

	select {
	case url := <-client.PostURLChan:
		if url != "http://example.com/messages" {
			t.Errorf("Expected http://example.com/messages, got %s", url)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Timed out waiting for endpoint url")
	}
}

func TestSSEClient_StartAndConnect(t *testing.T) {
	// Setup an HTTP server for SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		
		fmt.Fprintf(w, "event: endpoint\ndata: /mcp/messages\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	origGet := keyringGet
	defer func() { keyringGet = origGet }()
	mockToken := TokenData{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	tokenBytes, _ := json.Marshal(mockToken)
	keyringGet = func(service, user string) (string, error) {
		return string(tokenBytes), nil
	}

	client := NewSSEClient(&Config{
		RemoteURL:  server.URL + "/mcp/sse",
		UseIDToken: false,
	})

	client.Start()

	// Wait for connection to read and publish the post URL
	select {
	case u := <-client.PostURLChan:
		expected := server.URL + "/mcp/messages"
		if u != expected {
			t.Errorf("Expected %s, got %s", expected, u)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout waiting for SSE client to connect and receive endpoint event")
	}
}
