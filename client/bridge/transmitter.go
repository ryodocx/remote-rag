package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Transmitter struct {
	Config      *Config
	PostURLChan chan string
	StdinChan   chan string
	postClient  *http.Client
}

func NewTransmitter(config *Config, postURLChan chan string) *Transmitter {
	return &Transmitter{
		Config:      config,
		PostURLChan: postURLChan,
		StdinChan:   make(chan string),
		postClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// getToken は認証を実行し、設定に応じたトークンを取得します。
func (t *Transmitter) getToken() (string, error) {
	tokenData, err := GetValidToken()
	if err != nil {
		return "", err
	}

	token := tokenData.AccessToken
	if t.Config.UseIDToken {
		if tokenData.IDToken != "" {
			token = tokenData.IDToken
		}
	}
	return token, nil
}

// Start は標準入力の読み取りと、MCPサーバへのPOSTループを開始します。
func (t *Transmitter) Start() {
	// 標準入力を非同期で読み取る
	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		for scanner.Scan() {
			t.StdinChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		}
		close(t.StdinChan)
	}()

	var postURL string

	for {
		select {
		case msg, ok := <-t.StdinChan:
			if !ok {
				// Parent process closed stdin, exit normally
				return
			}
			if strings.TrimSpace(msg) == "" {
				continue
			}

			// SSEからエンドポイント情報を受け取るまで待機
			if postURL == "" {
				fmt.Fprintf(os.Stderr, "Waiting for endpoint event before sending message...\n")
				postURL = <-t.PostURLChan
			} else {
				// 新しいエンドポイントが来ている場合は更新する
				select {
				case postURL = <-t.PostURLChan:
				default:
				}
			}

			t.sendPostRequest(postURL, msg)
		}
	}
}

// sendPostRequest は指定されたエンドポイントにメッセージをPOSTします。
func (t *Transmitter) sendPostRequest(postURL, msg string) {
	token, err := t.getToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get token for POST: %v\n", err)
		return
	}

	postReq, err := http.NewRequest("POST", postURL, strings.NewReader(msg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create POST request: %v\n", err)
		return
	}
	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := t.postClient.Do(postReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "POST to MCP failed: %v\n", err)
		return
	}
	defer postResp.Body.Close()

	if postResp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "POST returned status %d\n", postResp.StatusCode)
	}

	io.Copy(io.Discard, postResp.Body)
}
