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
func SyncToAntigravity(acc *Account) error {
	if acc == nil {
		return nil
	}

	geminiDir, err := getGeminiDir()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(geminiDir, 0700)

	if acc.Type == TypeOAuth && acc.OAuth != nil {
		expiryTime := time.Now().Add(1 * time.Hour)
		if acc.OAuth.ExpiryDate > 0 {
			expiryTime = time.UnixMilli(acc.OAuth.ExpiryDate)
		}

		tokenType := acc.OAuth.TokenType
		if tokenType == "" {
			tokenType = "Bearer"
		}

		st := AntigravityStandaloneToken{
			Token: AntigravityOAuthToken{
				AccessToken:  acc.OAuth.AccessToken,
				TokenType:    tokenType,
				RefreshToken: acc.OAuth.RefreshToken,
				Expiry:       expiryTime.Format("2006-01-02T15:04:05.999999999Z07:00"),
			},
			AuthMethod: "consumer",
		}

		jsonData, err := json.Marshal(st)
		if err != nil {
			return err
		}

		// 1. Write ~/.gemini/jetski-standalone-oauth-token
		tokenPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
		if err := os.WriteFile(tokenPath, jsonData, 0600); err != nil {
			return fmt.Errorf("failed to write jetski token: %w", err)
		}

		// 2. Update macOS Keychain if on darwin
		if runtime.GOOS == "darwin" && os.Getenv("GEMINI_SWAP_SKIP_KEYCHAIN") != "1" {
			keychainVal := "go-keyring-base64:" + base64.StdEncoding.EncodeToString(jsonData)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			cmd := exec.CommandContext(ctx, "/usr/bin/security", "add-generic-password", "-U", "-s", "gemini", "-a", "antigravity", "-w", keychainVal)
			_ = cmd.Run()
			cancel()
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

	client := &http.Client{
		Timeout: 1500 * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	testPort := func(p int) bool {
		url := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/GetAuthStatus", p)
		req, rErr := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
		if rErr != nil {
			return false
		}
		req.Header.Set("x-codeium-csrf-token", csrfToken)
		req.Header.Set("Content-Type", "application/json")
		resp, dErr := client.Do(req)
		if dErr == nil {
			resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		}
		return false
	}

	if testPort(61955) {
		return 61955, csrfToken, pid, nil
	}

	lsofOut, lErr := exec.Command("lsof", "-nP", "-p", pid).Output()
	if lErr == nil {
		rePort := regexp.MustCompile(`:(\d+)\s+\(LISTEN\)`)
		for _, match := range rePort.FindAllStringSubmatch(string(lsofOut), -1) {
			if len(match) >= 2 {
				p, _ := strconv.Atoi(match[1])
				if p > 0 && testPort(p) {
					return p, csrfToken, pid, nil
				}
			}
		}
	}

	return 0, "", pid, fmt.Errorf("could not connect to antigravity language server")
}

// RestartAntigravityLanguageServer signals the Antigravity language_server to reload credentials.
func RestartAntigravityLanguageServer() error {
	port, csrf, pid, err := FindAntigravityServer()
	if err != nil {
		return nil
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
	if _, err := os.Stat(filePath); err == nil {
		// File already exists, don't overwrite
		return nil
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
		},
	}

	store := &StoreData{
		Version:         CurrentVersion,
		ActiveAccountID: acc.ID,
		Accounts:        []*Account{acc},
		ProxyPort:       DefaultPort,
		AutoRotate:      true,
		LastUpdated:     time.Now(),
	}

	return s.Save(store)
}
