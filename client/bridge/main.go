package main

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	RemoteURL  string
	UseIDToken bool
	NoAuth     bool
}

func ParseConfig() *Config {
	urlFlag := flag.String("url", "", "Remote MCP Server URL")
	noAuthFlag := flag.Bool("no-auth", false, "Bypass local keyring and OAuth checking")
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

	return &Config{
		RemoteURL:  remoteURL,
		UseIDToken: os.Getenv("USE_ID_TOKEN") == "true",
		NoAuth:     *noAuthFlag,
	}
}

func main() {
	config := ParseConfig()

	// 初回認証を実行し、トークンが取得可能か確認する
	if !config.NoAuth {
		_, err := GetValidToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get initial auth token: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Running in No-Auth mode bypass.\n")
	}

	fmt.Fprintf(os.Stderr, "Initial connection to remote MCP server established.\n")

	// SSEクライアントの初期化と開始
	sseClient := NewSSEClient(config)
	sseClient.Start()

	// 送信処理（標準入力から読み取りPOST送信）の初期化と開始
	// main ゴルーチンをここでブロックする
	transmitter := NewTransmitter(config, sseClient.PostURLChan)
	transmitter.Start()
}
