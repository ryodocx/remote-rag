package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	serviceName = "rrag-mcp" // Keychainに保存する際のサービス名
)

func getAccountName() string {
	profile := os.Getenv("RRAG_PROFILE")
	if profile != "" {
		return "oauth-token-" + profile
	}
	clientID := os.Getenv("OAUTH_CLIENT_ID")
	if clientID != "" {
		return "oauth-token-" + clientID
	}
	return "oauth-token"
}

// TokenData はアクセストークンおよびリフレッシュトークンの情報を保持し、JSONとしてシリアライズしてKeychainに保存されます
type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
}

var (
	discoveredAuthURL  string
	discoveredTokenURL string
)

func fetchOIDCDiscovery() {
	issuer := os.Getenv("OAUTH_ISSUER_URL")
	if issuer == "" || discoveredAuthURL != "" {
		return
	}

	url := issuer
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += ".well-known/openid-configuration"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to fetch OIDC discovery: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Warning: OIDC discovery returned status %d\n", resp.StatusCode)
		return
	}

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to parse OIDC discovery: %v\n", err)
		return
	}

	if auth, ok := config["authorization_endpoint"].(string); ok {
		discoveredAuthURL = auth
	}
	if token, ok := config["token_endpoint"].(string); ok {
		discoveredTokenURL = token
	}
}

func getOAuth2Config() *oauth2.Config {
	fetchOIDCDiscovery()

	authURL := os.Getenv("OAUTH_AUTH_URL")
	tokenURL := os.Getenv("OAUTH_TOKEN_URL")
	clientID := os.Getenv("OAUTH_CLIENT_ID")
	
	if authURL == "" && discoveredAuthURL != "" {
		authURL = discoveredAuthURL
	}
	if tokenURL == "" && discoveredTokenURL != "" {
		tokenURL = discoveredTokenURL
	}

	if authURL == "" {
		authURL = "https://mock-oauth-domain/authorize"
	}
	if tokenURL == "" {
		tokenURL = "https://mock-oauth-domain/token"
	}
	if clientID == "" {
		clientID = "mock-client-id"
	}

	return &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		Scopes: []string{"openid", "profile", "offline_access"},
		// RedirectURL はローカルで待ち受ける一時的なHTTPサーバーのポートに合わせて動的に設定します
	}
}

// generatePKCE は OAuth 2.0 PKCE (Proof Key for Code Exchange) フローに必要な Code Verifier と Code Challenge を生成します
func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	
	h := sha256.New()
	h.Write([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	return verifier, challenge, nil
}

// openBrowser は指定されたURLを、実行中のOSのデフォルトブラウザで開きます
func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

// Authenticate はブラウザを開いて認可サーバーでユーザーを認証させ、コールバックを受け取ってアクセストークンを取得します
func Authenticate() (string, error) {
	fmt.Fprintf(os.Stderr, "Authenticating with Identity Provider...\n")
	
	conf := getOAuth2Config()
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", fmt.Errorf("failed to generate PKCE: %v", err)
	}

	// 固定ポート (デフォルト 18080) または環境変数で指定されたポートでローカルサーバーを起動します
	portStr := os.Getenv("OAUTH_REDIRECT_PORT")
	if portStr == "" {
		portStr = "18080"
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+portStr)
	if err != nil {
		return "", fmt.Errorf("failed to bind local port: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	conf.RedirectURL = redirectURL

	authURL := conf.AuthCodeURL("state-token", oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	err = openBrowser(authURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open browser. Please open this URL manually:\n%s\n", authURL)
	}

	codeCh := make(chan string)
	errCh := make(chan error)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "state-token" {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			errCh <- fmt.Errorf("invalid state")
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Code not found", http.StatusBadRequest)
			errCh <- fmt.Errorf("code not found")
			return
		}
		fmt.Fprintf(w, "Authentication successful! You can close this window.")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)

	select {
	case code := <-codeCh:
		srv.Shutdown(context.Background())
		token, err := conf.Exchange(context.Background(), code,
			oauth2.SetAuthURLParam("code_verifier", verifier),
		)
		if err != nil {
			return "", err
		}
		
		saveToken(token)
		return token.AccessToken, nil
	case err := <-errCh:
		srv.Shutdown(context.Background())
		return "", err
	case <-time.After(5 * time.Minute):
		srv.Shutdown(context.Background())
		return "", fmt.Errorf("authentication timed out")
	}
}

// saveToken は取得したトークン情報をJSONにシリアライズし、OSネイティブのKeychainに暗号化して保存します
func saveToken(tok *oauth2.Token) {
	data := TokenData{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal token: %v\n", err)
		return
	}
	err = keyring.Set(serviceName, getAccountName(), string(bytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save to keyring: %v\n", err)
	}
}

// GetValidToken はKeychainからトークンを取得し、有効期限を確認します。トークンが存在しないか期限切れの場合は再認証を促します。
func GetValidToken() (string, error) {
	secret, err := keyring.Get(serviceName, getAccountName())
	if err != nil {
		// Keychainにトークンが見つからない場合は新規認証を実行
		return Authenticate()
	}

	var data TokenData
	if err := json.Unmarshal([]byte(secret), &data); err != nil {
		return Authenticate()
	}

	if time.Now().After(data.Expiry) {
		// Need to refresh
		if data.RefreshToken == "" {
			return Authenticate()
		}
		
		// TokenSourceを使用してトークンをリフレッシュ
		conf := getOAuth2Config()
		tok := &oauth2.Token{
			AccessToken:  data.AccessToken,
			RefreshToken: data.RefreshToken,
			Expiry:       data.Expiry,
		}
		
		tokenSource := conf.TokenSource(context.Background(), tok)
		newTok, err := tokenSource.Token()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to refresh token in background: %v\n", err)
			return Authenticate() // リフレッシュ失敗時は再認証
		}
		
		// リフレッシュ成功時、新しいトークンを保存
		saveToken(newTok)
		return newTok.AccessToken, nil
	}

	return data.AccessToken, nil
}
