package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var stdout io.Writer = os.Stdout

type SSEClient struct {
	Config      *Config
	PostURLChan chan string
}

func NewSSEClient(config *Config) *SSEClient {
	return &SSEClient{
		Config:      config,
		PostURLChan: make(chan string, 1),
	}
}

// getToken は認証を実行し、設定に応じたトークンを取得します。
func (c *SSEClient) getToken() (string, error) {
	tokenData, err := GetValidToken()
	if err != nil {
		return "", err
	}

	token := tokenData.AccessToken
	if c.Config.UseIDToken {
		if tokenData.IDToken == "" {
			fmt.Fprintf(os.Stderr, "Warning: USE_ID_TOKEN is true, but no ID token was found. Falling back to access token.\n")
		} else {
			token = tokenData.IDToken
		}
	}
	return token, nil
}

// Start はSSEストリームの接続と再接続ループを開始します。
func (c *SSEClient) Start() {
	go func() {
		retryDelay := 1 * time.Second
		for {
			token, err := c.getToken()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get token for SSE: %v\n", err)
				time.Sleep(retryDelay)
				continue
			}

			req, err := http.NewRequest("GET", c.Config.RemoteURL, nil)
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Accept", "text/event-stream")

				client := &http.Client{Timeout: 0}
				resp, err := client.Do(req)

				if err == nil && resp.StatusCode == http.StatusOK {
					retryDelay = 1 * time.Second // Reset backoff
					fmt.Fprintf(os.Stderr, "SSE Stream Connected.\n")

					c.readStream(resp.Body)
					resp.Body.Close()
				} else {
					if resp != nil {
						body, _ := io.ReadAll(resp.Body)
						fmt.Fprintf(os.Stderr, "SSE connection failed with status %d: %s\n", resp.StatusCode, string(body))
						resp.Body.Close()
					} else {
						fmt.Fprintf(os.Stderr, "SSE connection error: %v\n", err)
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "Failed to create SSE request: %v\n", err)
			}

			time.Sleep(retryDelay)
			retryDelay *= 2
			if retryDelay > 30*time.Second {
				retryDelay = 30 * time.Second
			}
		}
	}()
}

// readStream はレスポンスボディからSSEイベントを読み取ります。
func (c *SSEClient) readStream(body io.Reader) {
	reader := bufio.NewReader(body)
	var currentEvent string

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "SSE stream disconnected: %v\n", err)
			break
		}

		line = bytes.TrimSuffix(line, []byte("\n"))
		line = bytes.TrimSuffix(line, []byte("\r"))

		if bytes.HasPrefix(line, []byte("event: ")) {
			currentEvent = string(bytes.TrimPrefix(line, []byte("event: ")))
		} else if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))

			if currentEvent == "endpoint" {
				c.handleEndpointEvent(string(data))
			} else {
				// 標準出力へ中継
				stdout.Write(data)
				stdout.Write([]byte("\n"))
			}
		} else if len(line) == 0 {
			currentEvent = ""
		}
	}
}

// handleEndpointEvent はエンドポイント情報を受け取り、PostURLChan に送信します。
func (c *SSEClient) handleEndpointEvent(endpointURI string) {
	var pURL string
	if strings.HasPrefix(endpointURI, "http") {
		pURL = endpointURI
	} else {
		// remoteURL が https://domain/mcp/sse の場合、正しく https://domain/mcp/messages 等に解決する
		parsedRemote, err := url.Parse(c.Config.RemoteURL)
		if err == nil {
			parsedEndpoint, _ := url.Parse(endpointURI)
			pURL = parsedRemote.ResolveReference(parsedEndpoint).String()
		} else {
			// fallback
			baseURL := strings.TrimSuffix(c.Config.RemoteURL, "/")
			if !strings.HasPrefix(endpointURI, "/") {
				endpointURI = "/" + endpointURI
			}
			pURL = baseURL + endpointURI
		}
	}

	// 古いURLがチャネルに残っていれば捨てる
	select {
	case <-c.PostURLChan:
	default:
	}

	c.PostURLChan <- pURL
	fmt.Fprintf(os.Stderr, "Established POST endpoint: %s\n", pURL)
}
