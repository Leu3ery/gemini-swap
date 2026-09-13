package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gemini-swap/pkg/account"
)

const (
	AuthEndpoint     = "https://accounts.google.com/o/oauth2/v2/auth"
	TokenEndpoint    = "https://oauth2.googleapis.com/token"
	UserInfoEndpoint = "https://www.googleapis.com/oauth2/v2/userinfo"
)

func decodeMask(b []byte, key byte) string {
	res := make([]byte, len(b))
	for i, x := range b {
		res[i] = x ^ key
	}
	return string(res)
}

func getClientID() string {
	if v := os.Getenv("GEMINI_OAUTH_CLIENT_ID"); v != "" {
		return v
	}
	m := []byte{116, 122, 115, 112, 119, 119, 122, 114, 123, 113, 123, 119, 111, 45, 45, 122, 36, 54, 112, 45, 50, 48, 38, 48, 44, 50, 123, 39, 113, 35, 51, 36, 116, 35, 52, 113, 42, 47, 38, 43, 32, 115, 113, 119, 40, 108, 35, 50, 50, 49, 108, 37, 45, 45, 37, 46, 39, 55, 49, 39, 48, 33, 45, 44, 54, 39, 44, 54, 108, 33, 45, 47}
	return decodeMask(m, 0x42)
}

func getClientSecret() string {
	if v := os.Getenv("GEMINI_OAUTH_CLIENT_SECRET"); v != "" {
		return v
	}
	m := []byte{5, 13, 1, 17, 18, 26, 111, 118, 55, 10, 37, 15, 18, 47, 111, 115, 45, 117, 17, 41, 111, 37, 39, 20, 116, 1, 55, 119, 33, 46, 26, 4, 49, 58, 46}
	return decodeMask(m, 0x42)
}

func getAntigravityClientID() string {
	m := []byte{115, 114, 117, 115, 114, 114, 116, 114, 116, 114, 119, 123, 115, 111, 54, 47, 42, 49, 49, 43, 44, 112, 42, 112, 115, 46, 33, 48, 39, 112, 113, 119, 52, 54, 45, 46, 45, 40, 42, 118, 37, 118, 114, 113, 39, 50, 108, 35, 50, 50, 49, 108, 37, 45, 45, 37, 46, 39, 55, 49, 39, 48, 33, 45, 44, 54, 39, 44, 54, 108, 33, 45, 47}
	return decodeMask(m, 0x42)
}

func getAntigravityClientSecret() string {
	m := []byte{5, 13, 1, 17, 18, 26, 111, 9, 119, 122, 4, 21, 16, 118, 122, 116, 14, 38, 14, 8, 115, 47, 14, 0, 122, 49, 26, 1, 118, 56, 116, 51, 6, 3, 36}
	return decodeMask(m, 0x42)
}

var Scopes = []string{
	"openid",
	"email",
	"profile",
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// IDEScopes mirrors the scopes of a live Antigravity IDE session (seen in
// GetAuthStatus grantedScopes). cclog/aicode are registered on the IDE's own
// OAuth client — requesting them with the Gemini client yields 403.
var IDEScopes = []string{
	"openid",
	"email",
	"profile",
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/aicode",
	"https://www.googleapis.com/auth/experimentsandconfigs",
}

// ClientConfig selects which Google OAuth client a login flow uses.
// Tokens are bound to the client that minted them: only the IDE's own client
// produces refresh tokens the Antigravity language_server accepts.
type ClientConfig struct {
	ID     string
	Secret string
	Scopes []string
	// Tag is stored on Account.OAuth.Client ("gemini" or "antigravity").
	Tag string
}

// GeminiClient is the Gemini CLI OAuth client (durable for CLI/proxy/env,
// NOT refreshable by the Antigravity IDE).
func GeminiClient() ClientConfig {
	return ClientConfig{ID: getClientID(), Secret: getClientSecret(), Scopes: Scopes, Tag: "gemini"}
}

// AntigravityClient is the IDE's own OAuth client (durable everywhere,
// including inside the Antigravity IDE).
func AntigravityClient() ClientConfig {
	return ClientConfig{ID: getAntigravityClientID(), Secret: getAntigravityClientSecret(), Scopes: IDEScopes, Tag: "antigravity"}
}

type UserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generatePKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge
}

func openBrowser(targetUrl string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetUrl)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", targetUrl)
	default:
		cmd = exec.Command("xdg-open", targetUrl)
	}
	return cmd.Start()
}

func LoginWithBrowser(ctx context.Context) (*account.Account, error) {
	return LoginWithBrowserAs(ctx, GeminiClient())
}

func LoginWithBrowserAs(ctx context.Context, cfg ClientConfig) (*account.Account, error) {
	return LoginWithBrowserAsHint(ctx, cfg, "")
}

// LoginWithBrowserAsHint starts OAuth for a specific saved account. Google may
// still show an account chooser, but login_hint makes the intended identity the
// default and reduces accidental authorization of the currently open account.
func LoginWithBrowserAsHint(ctx context.Context, cfg ClientConfig, loginHint string) (*account.Account, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to open local port for oauth callback: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d", port)

	verifier, challenge := generatePKCE()
	state := generateRandomString(16)

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ID},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(cfg.Scopes, " ")},
		"access_type":           {"offline"},
		"prompt":                {"consent select_account"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	if loginHint != "" {
		params.Set("login_hint", loginHint)
	}
	authURL := AuthEndpoint + "?" + params.Encode()

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			queryState := r.URL.Query().Get("state")
			if queryState != state {
				http.Error(w, "Invalid state", http.StatusBadRequest)
				errChan <- errors.New("state parameter mismatch")
				return
			}
			code := r.URL.Query().Get("code")
			if code == "" {
				errMsg := r.URL.Query().Get("error")
				http.Error(w, "OAuth error: "+errMsg, http.StatusBadRequest)
				errChan <- fmt.Errorf("oauth error: %s", errMsg)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Gemini Swap Login Successful</title></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; text-align: center; padding-top: 80px; background-color: #0d1117; color: #c9d1d9;">
  <div style="display:inline-block; padding: 30px 50px; background-color: #161b22; border-radius: 12px; border: 1px solid #30363d;">
    <h2 style="color: #58a6ff; margin-bottom: 10px;">Authentication Successful!</h2>
    <p style="color: #8b949e; margin-bottom: 20px;">You have successfully signed in to Google. You can close this window and return to Gemini Swap.</p>
    <div style="font-size: 32px;">&#10004;</div>
  </div>
</body>
</html>`))

			codeChan <- code
		}),
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	fmt.Println("Attempting to open Google sign-in in your browser...")
	fmt.Printf("If it doesn't open automatically, visit this URL:\n\n%s\n\n", authURL)
	_ = openBrowser(authURL)

	var authCode string
	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return nil, ctx.Err()
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return nil, err
	case code := <-codeChan:
		authCode = code
		_ = server.Shutdown(context.Background())
	}

	return ExchangeCodeForAccountAs(authCode, redirectURI, verifier, cfg)
}

func LoginHeadless(ctx context.Context) (*account.Account, error) {
	return LoginHeadlessAs(ctx, GeminiClient())
}

func LoginHeadlessAs(ctx context.Context, cfg ClientConfig) (*account.Account, error) {
	// Headless flow for VPS.
	// The Gemini client accepts the hosted codeassist.google.com/authcode
	// redirect; any other client only accepts loopback redirects, so we use a
	// fixed localhost port and ask for the full redirect URL (works over SSH
	// without a tunnel: the code is in the address bar).
	redirectURI := "https://codeassist.google.com/authcode"
	headlessHint := "Paste the authorization code (or full redirect URL) here: "
	if cfg.Tag != "gemini" {
		redirectURI = "http://localhost:8493"
		headlessHint = "Paste the FULL redirect URL from your browser address bar here (http://localhost:8493/?code=...): "
	}
	verifier, challenge := generatePKCE()
	state := generateRandomString(16)

	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&access_type=offline&prompt=consent&code_challenge=%s&code_challenge_method=S256&state=%s",
		AuthEndpoint,
		url.QueryEscape(cfg.ID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(strings.Join(cfg.Scopes, " ")),
		url.QueryEscape(challenge),
		url.QueryEscape(state),
	)

	fmt.Println("\n=== Gemini Swap Remote/VPS Authentication ===")
	fmt.Println("1. Open the following link in any browser:")
	fmt.Printf("\n%s\n\n", authURL)
	fmt.Println("2. Sign in with your Google account and grant permissions.")
	fmt.Print("3. " + headlessHint)

	var rawInput string
	_, err := fmt.Scanln(&rawInput)
	if err != nil {
		return nil, err
	}

	authCode := strings.TrimSpace(rawInput)
	if strings.Contains(authCode, "code=") {
		u, err := url.Parse(authCode)
		if err == nil {
			authCode = u.Query().Get("code")
		}
	}

	return ExchangeCodeForAccountAs(authCode, redirectURI, verifier, cfg)
}

func ExchangeCodeForAccount(code, redirectURI, codeVerifier string) (*account.Account, error) {
	return ExchangeCodeForAccountAs(code, redirectURI, codeVerifier, GeminiClient())
}

func ExchangeCodeForAccountAs(code, redirectURI, codeVerifier string, cfg ClientConfig) (*account.Account, error) {
	values := url.Values{
		"code":          {code},
		"client_id":     {cfg.ID},
		"client_secret": {cfg.Secret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	if codeVerifier != "" {
		values.Set("code_verifier", codeVerifier)
	}

	resp, err := http.PostForm(TokenEndpoint, values)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	userInfo, err := FetchUserInfo(tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	expiryDate := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()

	acc := &account.Account{
		ID:        "google-" + userInfo.ID,
		Type:      account.TypeOAuth,
		Name:      userInfo.Name,
		Email:     userInfo.Email,
		Picture:   userInfo.Picture,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		OAuth: &account.OAuthData{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			TokenType:    tokenResp.TokenType,
			IDToken:      tokenResp.IDToken,
			ExpiryDate:   expiryDate,
			Scope:        tokenResp.Scope,
			Client:       cfg.Tag,
		},
	}
	if acc.Name == "" {
		acc.Name = userInfo.Email
	}

	return acc, nil
}

func RefreshToken(oauth *account.OAuthData) error {
	if oauth == nil || oauth.RefreshToken == "" {
		return errors.New("no refresh token available")
	}

	// Try the minting client first (known from provenance), then the other one.
	clientPairs := []struct {
		clientID     string
		clientSecret string
	}{
		{getClientID(), getClientSecret()},
		{getAntigravityClientID(), getAntigravityClientSecret()},
	}
	if oauth.Client == "antigravity" {
		clientPairs[0], clientPairs[1] = clientPairs[1], clientPairs[0]
	}

	var lastErr error
	for _, pair := range clientPairs {
		values := url.Values{
			"client_id":     {pair.clientID},
			"client_secret": {pair.clientSecret},
			"refresh_token": {oauth.RefreshToken},
			"grant_type":    {"refresh_token"},
		}

		resp, err := http.PostForm(TokenEndpoint, values)
		if err != nil {
			lastErr = fmt.Errorf("failed to refresh token: %w", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("refresh token request returned %d: %s", resp.StatusCode, string(body))
			continue
		}

		var tokenResp TokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			lastErr = fmt.Errorf("failed to decode refreshed token: %w", err)
			continue
		}

		oauth.AccessToken = tokenResp.AccessToken
		if tokenResp.RefreshToken != "" {
			oauth.RefreshToken = tokenResp.RefreshToken
		}
		if tokenResp.ExpiresIn > 0 {
			oauth.ExpiryDate = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()
		}

		return nil
	}

	return lastErr
}

func FetchUserInfo(accessToken string) (*UserInfo, error) {
	req, err := http.NewRequest("GET", UserInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request returned %d: %s", resp.StatusCode, string(body))
	}

	var u UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}
