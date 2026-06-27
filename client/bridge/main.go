package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	urlFlag := flag.String("url", "", "Remote MCP Server URL")
	flag.Parse()

	remoteURL := *urlFlag
	if remoteURL == "" {
		remoteURL = os.Getenv("MCP_REMOTE_URL")
	}
	if remoteURL == "" {
		fmt.Fprintf(os.Stderr, "Error: Remote MCP Server URL is not specified.\n")
		fmt.Fprintf(os.Stderr, "Please provide it via --url flag or MCP_REMOTE_URL environment variable.\n")
		os.Exit(1)
	}

	// 1. 認証を実行し、トークンを取得
	tokenData, err := GetValidToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get auth token: %v\n", err)
		os.Exit(1)
	}

	useIDToken := os.Getenv("USE_ID_TOKEN") == "true"
	token := tokenData.AccessToken
	if useIDToken {
		if tokenData.IDToken == "" {
			fmt.Fprintf(os.Stderr, "Warning: USE_ID_TOKEN is true, but no ID token was found. Falling back to access token.\n")
		} else {
			token = tokenData.IDToken
		}
	}

	fmt.Fprintf(os.Stderr, "Initial connection to remote MCP server established.\n")

	// 3. SSEイベントを受信し、標準出力 (stdout) へ書き出す
	postURLChan := make(chan string, 1)

	// 親プロセスからの標準入力 (stdin) を読み取り、リモートMCPへPOST送信
	postClient := &http.Client{Timeout: 30 * time.Second}
	scanner := bufio.NewScanner(os.Stdin)
	stdinChan := make(chan string)
	
	go func() {
		for scanner.Scan() {
			stdinChan <- scanner.Text()
		}
		close(stdinChan)
	}()

	var postURL string

	go func() {
		retryDelay := 1 * time.Second
		for {
			// Connect to SSE
			req, err := http.NewRequest("GET", remoteURL, nil)
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Accept", "text/event-stream")
				
				client := &http.Client{Timeout: 0}
				resp, err := client.Do(req)
				
				if err == nil && resp.StatusCode == http.StatusOK {
					retryDelay = 1 * time.Second // Reset backoff on successful connect
					fmt.Fprintf(os.Stderr, "SSE Stream Connected.\n")
					
					reader := bufio.NewReader(resp.Body)
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
								endpointURI := string(data)
								var pURL string
								if strings.HasPrefix(endpointURI, "http") {
									pURL = endpointURI
								} else {
									// remoteURL が https://domain/mcp/sse の場合、正しく https://domain/mcp/messages 等に解決する
									parsedRemote, err := url.Parse(remoteURL)
									if err == nil {
										parsedEndpoint, _ := url.Parse(endpointURI)
										pURL = parsedRemote.ResolveReference(parsedEndpoint).String()
									} else {
										// fallback
										baseURL := strings.TrimSuffix(remoteURL, "/")
										if !strings.HasPrefix(endpointURI, "/") {
											endpointURI = "/" + endpointURI
										}
										pURL = baseURL + endpointURI
									}
								}
								// drain previous URL if any
								select {
								case <-postURLChan:
								default:
								}
								postURLChan <- pURL
								fmt.Fprintf(os.Stderr, "Established POST endpoint: %s\n", pURL)
							} else {
								os.Stdout.Write(data)
								os.Stdout.Write([]byte("\n"))
							}
						} else if len(line) == 0 {
							currentEvent = ""
						}
					}
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
			}
			
			// Token might be expired, get a valid one
			tokenData, _ = GetValidToken()
			token = tokenData.AccessToken
			if useIDToken && tokenData.IDToken != "" {
				token = tokenData.IDToken
			}
			
			time.Sleep(retryDelay)
			retryDelay *= 2
			if retryDelay > 30*time.Second {
				retryDelay = 30 * time.Second
			}
		}
	}()

	for {
		select {
		case msg, ok := <-stdinChan:
			if !ok {
				if err := scanner.Err(); err != nil {
					fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				}
				// Parent process closed stdin, exit normally
				return
			}
			if strings.TrimSpace(msg) == "" {
				continue
			}

			// SSEからエンドポイント情報を受け取るまで待機
			if postURL == "" {
				fmt.Fprintf(os.Stderr, "Waiting for endpoint event before sending message...\n")
				postURL = <-postURLChan
			} else {
				// 新しいエンドポイントが来ている場合は更新する
				select {
				case postURL = <-postURLChan:
				default:
				}
			}

		postReq, err := http.NewRequest("POST", postURL, strings.NewReader(msg))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create POST request: %v\n", err)
			continue
		}
		postReq.Header.Set("Authorization", "Bearer "+token)
		postReq.Header.Set("Content-Type", "application/json")

		postResp, err := postClient.Do(postReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "POST to MCP failed: %v\n", err)
			continue
		}
		
		if postResp.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "POST returned status %d\n", postResp.StatusCode)
		}
		
		io.Copy(io.Discard, postResp.Body)
		postResp.Body.Close()
	}
}
