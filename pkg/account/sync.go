package account

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type GeminiOAuthCreds struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiryDate   int64  `json:"expiry_date,omitempty"`
}

type GeminiGoogleAccounts struct {
	Active string   `json:"active"`
	Old    []string `json:"old"`
}

type AntigravityOAuthToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Expiry       string `json:"expiry,omitempty"`
}

type AntigravityStandaloneToken struct {
	Token      AntigravityOAuthToken `json:"token"`
	AuthMethod string                `json:"auth_method"`
}

func getGeminiDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini"), nil
}

func SyncToGeminiCLI(acc *Account) error {
	if acc == nil {
		return nil
	}

	geminiDir, err := getGeminiDir()
	if err != nil {
		return err
	}

	// Always ensure .gemini dir exists
	_ = os.MkdirAll(geminiDir, 0700)

	if acc.Type == TypeOAuth && acc.OAuth != nil {
		credsPath := filepath.Join(geminiDir, "oauth_creds.json")
		accountsPath := filepath.Join(geminiDir, "google_accounts.json")

		creds := GeminiOAuthCreds{
			AccessToken:  acc.OAuth.AccessToken,
			RefreshToken: acc.OAuth.RefreshToken,
			Scope:        acc.OAuth.Scope,
			TokenType:    acc.OAuth.TokenType,
			IDToken:      acc.OAuth.IDToken,
			ExpiryDate:   acc.OAuth.ExpiryDate,
		}
		if creds.TokenType == "" {
			creds.TokenType = "Bearer"
		}

		credsData, err := json.MarshalIndent(creds, "", "  ")
		if err == nil {
			_ = os.WriteFile(credsPath, credsData, 0600)
		}

		if acc.Email != "" {
			var accData GeminiGoogleAccounts
			raw, rErr := os.ReadFile(accountsPath)
			if rErr == nil {
				_ = json.Unmarshal(raw, &accData)
			}
			if accData.Active != "" && accData.Active != acc.Email {
				// add previous to old
				found := false
				for _, o := range accData.Old {
					if o == accData.Active {
						found = true
						break
					}
				}
				if !found {
					accData.Old = append(accData.Old, accData.Active)
				}
			}
			accData.Active = acc.Email
			if accData.Old == nil {
				accData.Old = []string{}
			}
			accBytes, aErr := json.MarshalIndent(accData, "", "  ")
			if aErr == nil {
				_ = os.WriteFile(accountsPath, accBytes, 0644)
			}
		}
	}

	// Also write ~/.gemini-swap/current_env.sh for easy shell sourcing
	baseDir, _ := GetDefaultBaseDir()
	if baseDir != "" {
		envPath := filepath.Join(baseDir, "current_env.sh")
		var envContent string
		if acc.Type == TypeAPIKey {
			envContent = fmt.Sprintf("export GEMINI_API_KEY=\"%s\"\nexport GOOGLE_API_KEY=\"%s\"\n", acc.APIKey, acc.APIKey)
		} else if acc.OAuth != nil {
			envContent = fmt.Sprintf("export GOOGLE_GENAI_USE_GCA=true\nexport GOOGLE_CLOUD_ACCESS_TOKEN=\"%s\"\n", acc.OAuth.AccessToken)
		}
		_ = os.WriteFile(envPath, []byte(envContent), 0600)
	}

	return nil
}

// SyncToAntigravity synchronizes the active account credentials with Google Antigravity IDE.
func standaloneTokenToOAuth(st AntigravityStandaloneToken) *OAuthData {
	oauth := &OAuthData{
		AccessToken:  st.Token.AccessToken,
		RefreshToken: st.Token.RefreshToken,
		TokenType:    st.Token.TokenType,
	}
	if oauth.TokenType == "" {
		oauth.TokenType = "Bearer"
	}
	if st.Token.Expiry != "" {
		if t, err := time.Parse(time.RFC3339, st.Token.Expiry); err == nil {
			oauth.ExpiryDate = t.UnixMilli()
		}
	}
	return oauth
}

// writeAntigravityToken writes keychain + jetski file from OAuth data.
// It returns the raw keychain value written (for post-switch verification).
func writeAntigravityToken(oauth *OAuthData) (string, error) {
	if oauth == nil {
		return "", fmt.Errorf("nil oauth data")
	}

	geminiDir, err := getGeminiDir()
	if err != nil {
		return "", err
	}
	_ = os.MkdirAll(geminiDir, 0700)

	expiryTime := time.Now().Add(1 * time.Hour)
	if oauth.ExpiryDate > 0 {
		expiryTime = time.UnixMilli(oauth.ExpiryDate)
	}

	tokenType := oauth.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	st := AntigravityStandaloneToken{
		Token: AntigravityOAuthToken{
			AccessToken:  oauth.AccessToken,
			TokenType:    tokenType,
			RefreshToken: oauth.RefreshToken,
			Expiry:       expiryTime.Format("2006-01-02T15:04:05.999999999Z07:00"),
		},
		AuthMethod: "consumer",
	}

	jsonData, err := json.Marshal(st)
	if err != nil {
		return "", err
	}

	// 1. Write ~/.gemini/jetski-standalone-oauth-token
	tokenPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
	if err := os.WriteFile(tokenPath, jsonData, 0600); err != nil {
		return "", fmt.Errorf("failed to write jetski token: %w", err)
	}

	// 2. Update macOS Keychain if on darwin
	keychainVal := "go-keyring-base64:" + base64.StdEncoding.EncodeToString(jsonData)
	if runtime.GOOS == "darwin" && os.Getenv("GEMINI_SWAP_SKIP_KEYCHAIN") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/usr/bin/security", "add-generic-password", "-U", "-s", "gemini", "-a", "antigravity", "-w", keychainVal)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to update keychain: %w", err)
		}
	}

	return keychainVal, nil
}

func SyncToAntigravity(acc *Account) error {
	if acc == nil {
		return nil
	}

	if acc.Type == TypeOAuth && acc.OAuth != nil {
		// NEVER write Gemini-client-bound tokens into the IDE session:
		// the language_server cannot refresh them (unauthorized_client) and
		// drops the whole session (Welcome/Sign-in screen). Proven live.
		if acc.OAuth.Client != "antigravity" {
			return nil
		}
		_, err := writeAntigravityToken(acc.OAuth)
		return err
	}

	return nil
}

// ReadAntigravityKeychainToken reads the live session directly from the macOS
// Keychain (no file fallback). It returns the parsed token and the raw stored
// value (used to detect whether the IDE accepted or replaced our write).
func ReadAntigravityKeychainToken() (*OAuthData, string, error) {
	if runtime.GOOS != "darwin" {
		return nil, "", fmt.Errorf("keychain only available on darwin")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", "gemini", "-a", "antigravity", "-w")
	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("no antigravity keychain entry: %w", err)
	}
	val := strings.TrimSpace(string(out))
	const prefix = "go-keyring-base64:"
	b64 := val
	if strings.HasPrefix(val, prefix) {
		b64 = strings.TrimPrefix(val, prefix)
	}
	raw, dErr := base64.StdEncoding.DecodeString(b64)
	if dErr != nil {
		return nil, "", fmt.Errorf("failed to decode keychain entry: %w", dErr)
	}
	var st AntigravityStandaloneToken
	if jErr := json.Unmarshal(raw, &st); jErr != nil {
		return nil, "", fmt.Errorf("failed to parse keychain entry: %w", jErr)
	}
	if st.Token.AccessToken == "" && st.Token.RefreshToken == "" {
		return nil, "", fmt.Errorf("keychain entry is empty")
	}
	return standaloneTokenToOAuth(st), val, nil
}

func readJetskiToken() (*OAuthData, []byte, error) {
	geminiDir, err := getGeminiDir()
	if err != nil {
		return nil, nil, err
	}
	raw, err := os.ReadFile(filepath.Join(geminiDir, "jetski-standalone-oauth-token"))
	if err != nil {
		return nil, nil, err
	}
	var st AntigravityStandaloneToken
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, nil, fmt.Errorf("failed to parse jetski token: %w", err)
	}
	if st.Token.AccessToken == "" && st.Token.RefreshToken == "" {
		return nil, nil, fmt.Errorf("jetski token is empty")
	}
	return standaloneTokenToOAuth(st), raw, nil
}

// ReadAntigravitySessionToken reads the live Antigravity IDE session token.
// Source of truth order: macOS Keychain entry (written by Antigravity itself
// on every refresh) first, ~/.gemini/jetski-standalone-oauth-token file as fallback.
// NOTE: this may differ from ~/.gemini/oauth_creds.json (Gemini CLI session) —
// the user can legitimately be logged into different Google accounts in each tool.
func ReadAntigravitySessionToken() (*OAuthData, error) {
	if oauth, _, err := ReadAntigravityKeychainToken(); err == nil {
		return oauth, nil
	}
	// Keychain unavailable — fall through to file
	oauth, _, err := readJetskiToken()
	return oauth, err
}

type antigravityBackup struct {
	CreatedAt    time.Time `json:"created_at"`
	KeychainVal  string    `json:"keychain_value,omitempty"`
	JetskiRaw    string    `json:"jetski_raw_base64,omitempty"`
	AccessPrefix string    `json:"access_prefix,omitempty"`
}

func backupsDir() (string, error) {
	baseDir, err := GetDefaultBaseDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(baseDir, "antigravity-backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// BackupAntigravitySession snapshots the current IDE session (keychain + jetski
// file) before it gets overwritten by a switch, so a rejected switch can be
// rolled back with RestoreAntigravitySession.
func BackupAntigravitySession() (string, error) {
	dir, err := backupsDir()
	if err != nil {
		return "", err
	}

	b := antigravityBackup{CreatedAt: time.Now()}
	if oauth, raw, err := ReadAntigravityKeychainToken(); err == nil {
		b.KeychainVal = raw
		if len(oauth.AccessToken) > 12 {
			b.AccessPrefix = oauth.AccessToken[:12]
		}
	}
	if _, jetskiRaw, err := readJetskiToken(); err == nil {
		b.JetskiRaw = base64.StdEncoding.EncodeToString(jetskiRaw)
	}
	if b.KeychainVal == "" && b.JetskiRaw == "" {
		return "", fmt.Errorf("nothing to back up: no antigravity session found")
	}

	path := filepath.Join(dir, fmt.Sprintf("backup-%d.json", time.Now().Unix()))
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func latestBackupPath() (string, error) {
	dir, err := backupsDir()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	best := ""
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if e.Name() > best {
			best = e.Name()
		}
	}
	if best == "" {
		return "", fmt.Errorf("no antigravity backups found in %s", dir)
	}
	return filepath.Join(dir, best), nil
}

// RestoreAntigravitySession writes a previously backed-up IDE session back to
// keychain + jetski file. Empty path restores the latest backup.
func RestoreAntigravitySession(path string) error {
	if path == "" {
		var err error
		path, err = latestBackupPath()
		if err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read backup %s: %w", path, err)
	}
	var b antigravityBackup
	if err := json.Unmarshal(raw, &b); err != nil {
		return fmt.Errorf("failed to parse backup %s: %w", path, err)
	}

	geminiDir, err := getGeminiDir()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(geminiDir, 0700)

	if b.JetskiRaw != "" {
		jetski, dErr := base64.StdEncoding.DecodeString(b.JetskiRaw)
		if dErr != nil {
			return fmt.Errorf("corrupt backup (jetski): %w", dErr)
		}
		if err := os.WriteFile(filepath.Join(geminiDir, "jetski-standalone-oauth-token"), jetski, 0600); err != nil {
			return err
		}
	}
	if b.KeychainVal != "" && runtime.GOOS == "darwin" && os.Getenv("GEMINI_SWAP_SKIP_KEYCHAIN") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/usr/bin/security", "add-generic-password", "-U", "-s", "gemini", "-a", "antigravity", "-w", b.KeychainVal)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to restore keychain: %w", err)
		}
	}
	return nil
}

// FindAntigravityServer finds the running Antigravity language_server process, port, and CSRF token.
func FindAntigravityServer() (port int, csrfToken string, pid string, err error) {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		return 0, "", "", err
	}

	reCsrf := regexp.MustCompile(`--csrf_token\s+([a-fA-F0-9-]+)`)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "language_server") && strings.Contains(line, "--csrf_token") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pid = fields[1]
				m := reCsrf.FindStringSubmatch(line)
				if len(m) >= 2 {
					csrfToken = m[1]
					break
				}
			}
		}
	}

	if pid == "" || csrfToken == "" {
		return 0, "", "", fmt.Errorf("antigravity language_server not running")
	}

	clientTLS := &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	clientPlain := &http.Client{
		Timeout: 4 * time.Second,
	}

	testPort := func(p int) bool {
		// The language_server exposes HTTPS on one port and plain HTTP on
		// another (ports are dynamic: --https_server_port 0). Try HTTPS first,
		// then fall back to plain HTTP.
		for _, tc := range []struct {
			scheme string
			client *http.Client
		}{
			{"https", clientTLS},
			{"http", clientPlain},
		} {
			url := fmt.Sprintf("%s://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/GetAuthStatus", tc.scheme, p)
			req, rErr := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
			if rErr != nil {
				continue
			}
			req.Header.Set("x-codeium-csrf-token", csrfToken)
			req.Header.Set("Content-Type", "application/json")
			resp, dErr := tc.client.Do(req)
			if dErr == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return true
				}
			}
		}
		return false
	}

	// Collect candidate ports from lsof first (ports are dynamic), then also
	// probe the legacy default. Deduplicate to avoid double probing.
	candidates := []int{}
	seen := map[int]bool{}
	lsofOut, lErr := exec.Command("lsof", "-nP", "-p", pid).Output()
	if lErr == nil {
		rePort := regexp.MustCompile(`:(\d+)\s+\(LISTEN\)`)
		for _, match := range rePort.FindAllStringSubmatch(string(lsofOut), -1) {
			if len(match) >= 2 {
				if p, _ := strconv.Atoi(match[1]); p > 0 && !seen[p] {
					seen[p] = true
					candidates = append(candidates, p)
				}
			}
		}
	}
	if !seen[61955] {
		candidates = append(candidates, 61955)
	}

	for _, p := range candidates {
		if testPort(p) {
			return p, csrfToken, pid, nil
		}
	}

	return 0, "", pid, fmt.Errorf("could not connect to antigravity language server")
}

// GetAntigravityUserEmail asks the running language server which Google
// account it is actually serving. Unlike reading our own token/keychain write,
// this only changes after Antigravity has finished loading the new session.
func GetAntigravityUserEmail() (string, error) {
	port, csrf, _, err := FindAntigravityServer()
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	url := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/GetUserStatus", port)
	req, err := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-codeium-csrf-token", csrf)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GetUserStatus returned %s", resp.Status)
	}

	var status struct {
		UserStatus struct {
			Email string `json:"email"`
		} `json:"userStatus"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return "", fmt.Errorf("failed to decode GetUserStatus: %w", err)
	}
	if status.UserStatus.Email == "" {
		return "", fmt.Errorf("GetUserStatus did not return an account email")
	}
	return status.UserStatus.Email, nil
}

// RestartAntigravityLanguageServer signals the Antigravity language_server to reload credentials.
// Returns nil when the IDE is not running (nothing to do). Returns an error
// when the server was found but could not be signaled, so callers can warn
// the user to reload the Antigravity window manually.
func RestartAntigravityLanguageServer() error {
	if os.Getenv("GEMINI_SWAP_NO_RESTART") == "1" {
		return nil
	}
	port, csrf, pid, err := FindAntigravityServer()
	if err != nil {
		if pid == "" {
			return nil // IDE not running — nothing to reload
		}
		return err
	}

	// 1. Try graceful RPC Restart
	if port > 0 && csrf != "" {
		client := &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		url := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/Restart", port)
		req, rErr := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
		if rErr == nil {
			req.Header.Set("x-codeium-csrf-token", csrf)
			req.Header.Set("Content-Type", "application/json")
			resp, dErr := client.Do(req)
			if dErr == nil {
				resp.Body.Close()
			}
		}
	}

	// 2. Wait up to 1.5s to see if process exited
	time.Sleep(500 * time.Millisecond)
	if pid != "" {
		chk := exec.Command("kill", "-0", pid)
		if chk.Run() == nil {
			_ = exec.Command("kill", "-TERM", pid).Run()
		}
	}

	return nil
}

func (s *Storage) autoImportExisting() error {
	filePath := s.GetFilePath()
	if raw, err := os.ReadFile(filePath); err == nil {
		// File exists: only skip import when it already holds accounts.
		// An empty store (e.g. after removing the last account) must heal
		// itself from ~/.gemini/oauth_creds.json instead of staying empty forever.
		var existing StoreData
		if json.Unmarshal(raw, &existing) == nil && len(existing.Accounts) > 0 {
			return nil
		}
	}

	geminiDir, err := getGeminiDir()
	if err != nil {
		return err
	}

	credsPath := filepath.Join(geminiDir, "oauth_creds.json")
	credsData, err := os.ReadFile(credsPath)
	if err != nil {
		return nil // No existing gemini creds
	}

	var oauthCreds GeminiOAuthCreds
	if err := json.Unmarshal(credsData, &oauthCreds); err != nil {
		return nil
	}

	if oauthCreds.AccessToken == "" && oauthCreds.RefreshToken == "" {
		return nil
	}

	// Read email from google_accounts.json if present
	email := "google-account-1"
	accountsPath := filepath.Join(geminiDir, "google_accounts.json")
	if raw, err := os.ReadFile(accountsPath); err == nil {
		var gAcc GeminiGoogleAccounts
		if json.Unmarshal(raw, &gAcc) == nil && gAcc.Active != "" {
			email = gAcc.Active
		}
	}

	acc := &Account{
		ID:        "google-" + fmt.Sprintf("%d", time.Now().Unix()),
		Type:      TypeOAuth,
		Name:      email,
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		OAuth: &OAuthData{
			AccessToken:  oauthCreds.AccessToken,
			RefreshToken: oauthCreds.RefreshToken,
			TokenType:    oauthCreds.TokenType,
			IDToken:      oauthCreds.IDToken,
			ExpiryDate:   oauthCreds.ExpiryDate,
			Scope:        oauthCreds.Scope,
			Client:       "gemini",
		},
	}

	// Preserve existing store settings when healing an empty store.
	var store *StoreData
	if raw, err := os.ReadFile(filePath); err == nil {
		var existing StoreData
		if json.Unmarshal(raw, &existing) == nil {
			store = &existing
		}
	}
	if store == nil {
		store = &StoreData{
			Version:    CurrentVersion,
			ProxyPort:  DefaultPort,
			AutoRotate: true,
		}
	}
	// Don't duplicate if the same email is already stored.
	for _, existing := range store.Accounts {
		if existing.Email != "" && existing.Email == acc.Email {
			if store.ActiveAccountID == "" {
				store.ActiveAccountID = existing.ID
			}
			return s.Save(store)
		}
	}
	store.Accounts = append(store.Accounts, acc)
	if store.ActiveAccountID == "" {
		store.ActiveAccountID = acc.ID
	}
	if store.ProxyPort <= 0 {
		store.ProxyPort = DefaultPort
	}

	return s.Save(store)
}
