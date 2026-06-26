package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	remoteURL := os.Getenv("MCP_REMOTE_URL")
	if remoteURL == "" {
		remoteURL = "http://localhost:8080" // Caddy proxy default
	}

	// 1. Authenticate and get Token
	token, err := GetValidToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get auth token: %v\n", err)
		os.Exit(1)
	}

	// 2. Connect to SSE Endpoint
	sseURL := remoteURL + "/sse"
	req, err := http.NewRequest("GET", sseURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create SSE request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		Timeout: 0, // No timeout for SSE
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to SSE: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "SSE connection failed with status %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Connected to remote MCP server.\n")

	// 3. Read SSE events and write to Stdout
	// Parse the POST endpoint from the 'endpoint' event.
	var postURL string
	var currentEvent string

	go func() {
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "SSE stream ended: %v\n", err)
				os.Exit(0)
			}
			
			line = bytes.TrimSuffix(line, []byte("\n"))
			line = bytes.TrimSuffix(line, []byte("\r"))

			if bytes.HasPrefix(line, []byte("event: ")) {
				currentEvent = string(bytes.TrimPrefix(line, []byte("event: ")))
			} else if bytes.HasPrefix(line, []byte("data: ")) {
				data := bytes.TrimPrefix(line, []byte("data: "))
				
				if currentEvent == "endpoint" {
					// The data contains the URI for the POST endpoint.
					endpointURI := string(data)
					// If it's a relative path, append to remoteURL, else use as is
					if strings.HasPrefix(endpointURI, "http") {
						postURL = endpointURI
					} else {
						// Ensure trailing slash logic
						baseURL := strings.TrimSuffix(remoteURL, "/")
						if !strings.HasPrefix(endpointURI, "/") {
							endpointURI = "/" + endpointURI
						}
						postURL = baseURL + endpointURI
					}
					fmt.Fprintf(os.Stderr, "Established POST endpoint: %s\n", postURL)
				} else {
					// Forward JSON-RPC payload to stdio
					os.Stdout.Write(data)
					os.Stdout.Write([]byte("\n"))
				}
			} else if len(line) == 0 {
				// Empty line means end of event
				currentEvent = ""
			}
		}
	}()

	// 4. Read from Stdin and POST to remote MCP
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		if strings.TrimSpace(msg) == "" {
			continue
		}

		if postURL == "" {
			fmt.Fprintf(os.Stderr, "Waiting for endpoint event before sending message...\n")
			time.Sleep(1 * time.Second) // basic backoff if user types too fast
			if postURL == "" {
				continue
			}
		}

		postReq, err := http.NewRequest("POST", postURL, strings.NewReader(msg))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create POST request: %v\n", err)
			continue
		}
		postReq.Header.Set("Authorization", "Bearer "+token)
		postReq.Header.Set("Content-Type", "application/json")

		postClient := &http.Client{Timeout: 30 * time.Second}
		postResp, err := postClient.Do(postReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "POST to MCP failed: %v\n", err)
			continue
		}
		postResp.Body.Close()
		
		if postResp.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "POST returned status %d\n", postResp.StatusCode)
		}
	}
}
