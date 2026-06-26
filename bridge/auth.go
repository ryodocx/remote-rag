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
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	serviceName = "remote-rag-mcp"
	accountName = "oauth-token"
)

type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
}

func getOAuth2Config() *oauth2.Config {
	authURL := os.Getenv("OAUTH_AUTH_URL")
	tokenURL := os.Getenv("OAUTH_TOKEN_URL")
	clientID := os.Getenv("OAUTH_CLIENT_ID")
	
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
		// RedirectURL will be set dynamically based on the local listener port
	}
}

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

func Authenticate() (string, error) {
	fmt.Fprintf(os.Stderr, "Authenticating with Identity Provider...\n")
	
	conf := getOAuth2Config()
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", fmt.Errorf("failed to generate PKCE: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
	err = keyring.Set(serviceName, accountName, string(bytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save to keyring: %v\n", err)
	}
}

func GetValidToken() (string, error) {
	secret, err := keyring.Get(serviceName, accountName)
	if err != nil {
		// Not found, authenticate
		return Authenticate()
	}

	var data TokenData
	if err := json.Unmarshal([]byte(secret), &data); err != nil {
		return Authenticate()
	}

	if time.Now().After(data.Expiry) {
		// Need to refresh. For simplicity, just re-authenticate if no refresh token logic
		if data.RefreshToken == "" {
			return Authenticate()
		}
		// In a full implementation, we would use oauth2 to refresh the token here.
		// Mocking re-auth for now:
		return Authenticate()
	}

	return data.AccessToken, nil
}
