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
		// 未指定時のデフォルトはCaddyのリバースプロキシ (ローカル環境向け)
		remoteURL = "http://localhost:8080" 
	}

	// 1. 認証を実行し、アクセストークンを取得
	token, err := GetValidToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get auth token: %v\n", err)
		os.Exit(1)
	}

	// 2. リモートMCPサーバーのSSE (Server-Sent Events) エンドポイントへ接続
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

	// 3. SSEイベントを受信し、標準出力 (stdout) へ書き出す
	// 初期接続時の 'endpoint' イベントから、後続のPOSTリクエスト先URLを解析・保持します
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
					// データペイロードにはPOSTリクエスト用のURIが含まれています
					endpointURI := string(data)
					// 絶対パスか相対パスかを判定して完全なURLを構築
					if strings.HasPrefix(endpointURI, "http") {
						postURL = endpointURI
					} else {
						// トレイリングスラッシュの重複を防ぐ処理
						baseURL := strings.TrimSuffix(remoteURL, "/")
						if !strings.HasPrefix(endpointURI, "/") {
							endpointURI = "/" + endpointURI
						}
						postURL = baseURL + endpointURI
					}
					fmt.Fprintf(os.Stderr, "Established POST endpoint: %s\n", postURL)
				} else {
					// 'endpoint' 以外のデータ（主にメッセージ）は、JSON-RPCとして標準出力へそのまま転送
					os.Stdout.Write(data)
					os.Stdout.Write([]byte("\n"))
				}
			} else if len(line) == 0 {
				// 空行はイベントチャンクの終了を意味します
				currentEvent = ""
			}
		}
	}()

	// 4. 親プロセスからの標準入力 (stdin) を読み取り、リモートMCPへPOST送信
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		if strings.TrimSpace(msg) == "" {
			continue
		}

		// SSEからエンドポイント情報を受け取るまで待機
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
