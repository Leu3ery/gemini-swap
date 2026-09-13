package account

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSyncToAntigravity(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GEMINI_SWAP_NO_RESTART", "1")
	t.Setenv("GEMINI_SWAP_SKIP_KEYCHAIN", "1")

	acc := &Account{
		ID:        "google-123456",
		Type:      TypeOAuth,
		Name:      "Test User",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		OAuth: &OAuthData{
			AccessToken:  "test-access-token-123",
			RefreshToken: "test-refresh-token-456",
			TokenType:    "Bearer",
			ExpiryDate:   time.Now().Add(1 * time.Hour).UnixMilli(),
			Scope:        "openid email profile",
		},
	}

	if err := SyncToAntigravity(acc); err != nil {
		t.Fatalf("SyncToAntigravity failed: %v", err)
	}

	tokenPath := filepath.Join(tmpHome, ".gemini", "jetski-standalone-oauth-token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read token file: %v", err)
	}

	var st AntigravityStandaloneToken
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("Failed to unmarshal AntigravityStandaloneToken: %v", err)
	}

	if st.AuthMethod != "consumer" {
		t.Errorf("Expected auth_method 'consumer', got %q", st.AuthMethod)
	}
	if st.Token.AccessToken != "test-access-token-123" {
		t.Errorf("Expected access_token 'test-access-token-123', got %q", st.Token.AccessToken)
	}
	if st.Token.RefreshToken != "test-refresh-token-456" {
		t.Errorf("Expected refresh_token 'test-refresh-token-456', got %q", st.Token.RefreshToken)
	}
	if st.Token.TokenType != "Bearer" {
		t.Errorf("Expected token_type 'Bearer', got %q", st.Token.TokenType)
	}
	if st.Token.Expiry == "" {
		t.Error("Expected non-empty expiry timestamp")
	}
}
