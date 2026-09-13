package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
