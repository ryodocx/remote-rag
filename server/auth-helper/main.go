package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	cacheInstance      Cache  // キャッシュインターフェースのインスタンス
	oauthIntrospectURL string // OAuth2.0 Token Introspection エンドポイントのURL
	oauthClientID      string // Introspection用のクライアントID (Basic認証用)
	oauthClientSecret  string // Introspection用のクライアントシークレット (Basic認証用)
	ctx                = context.Background() // グローバルなコンテキスト
)

func init() {
	// 環境変数に基づいて使用するキャッシュ機構を決定します
	cacheType := os.Getenv("CACHE_TYPE")
	if cacheType == "memory" {
		log.Println("Using In-Memory Cache")
		cacheInstance = NewMemoryCache()
	} else {
		log.Println("Using Redis Cache")
		cacheInstance = NewRedisCache(os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"))
	}

	// Introspection用の設定を環境変数から取得します
	oauthIntrospectURL = os.Getenv("OAUTH_INTROSPECT_URL")
	oauthClientID = os.Getenv("OAUTH_CLIENT_ID")
	oauthClientSecret = os.Getenv("OAUTH_CLIENT_SECRET")

	if oauthIntrospectURL == "" || oauthClientID == "" || oauthClientSecret == "" {
		log.Println("WARNING: OAUTH_INTROSPECT_URL, OAUTH_CLIENT_ID, or OAUTH_CLIENT_SECRET is missing.")
	}
}

// hashToken は平文のトークンをSHA-256でハッシュ化し、16進数文字列を返します。
// キャッシュにトークンをそのまま保存するセキュリティリスクを避けるために使用します。
func hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

// introspectToken は RFC 7662 に基づき、認可サーバーに対してトークンのオンライン検証を行います。
// 有効なトークンであれば true を、無効であれば false を返します。
func introspectToken(token string) (bool, error) {
	if oauthIntrospectURL == "" {
		if os.Getenv("MOCK_AUTH") == "true" {
			log.Println("Mocking introspection: Returning true (MOCK_AUTH is true)")
			return true, nil
		}
		log.Println("OAUTH_INTROSPECT_URL is missing and MOCK_AUTH is not true. Rejecting token.")
		return false, nil
	}

	// x-www-form-urlencoded 形式でパラメータを準備します
	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")

	req, err := http.NewRequest("POST", oauthIntrospectURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Basic Auth
	auth := oauthClientID + ":" + oauthClientSecret
	basicAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+basicAuth)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Introspection failed with status %d: %s", resp.StatusCode, string(body))
		return false, nil
	}

	var result struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.Active, nil
}

// authHandler は Caddy の forward_auth から呼び出される認証ハンドラです。
func authHandler(w http.ResponseWriter, r *http.Request) {
	// 1. AuthorizationヘッダーからBearerトークンを抽出
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized) // トークンがない場合は即座に401を返す
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 2. トークンをハッシュ化して安全なキーを作成
	tokenHash := hashToken(token)

	// 3. キャッシュ（RedisまたはMemory）を参照
	val, err := cacheInstance.Get(ctx, tokenHash)
	if err == nil && val == "valid" {
		// キャッシュヒット：トークンは有効であるため Caddy に 200 OK を返す
		w.WriteHeader(http.StatusOK)
		return
	}

	// 4. キャッシュミス：認可サーバーへオンライン検証 (Introspection API) を実行
	active, err := introspectToken(token)
	if err != nil {
		log.Printf("Error introspecting token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if active {
		// 5. 検証成功：結果を60秒間キャッシュし、200 OK を返す
		cacheInstance.Set(ctx, tokenHash, "valid", 60*time.Second)
		w.WriteHeader(http.StatusOK)
		return
	}

	// トークンが無効な場合
	w.WriteHeader(http.StatusUnauthorized)
}

func main() {
	http.HandleFunc("/auth", authHandler)
	port := "8000"
	log.Printf("Auth Helper starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
