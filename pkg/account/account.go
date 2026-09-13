package account

import (
	"time"
)

type AccountType string

const (
	TypeOAuth  AccountType = "oauth"
	TypeAPIKey AccountType = "api_key"
)

type OAuthData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiryDate   int64  `json:"expiry_date,omitempty"` // Unix ms
	Scope        string `json:"scope,omitempty"`
	// Client marks which OAuth client minted the refresh token:
	// "gemini" (Gemini CLI / gemini-swap login) or "antigravity" (IDE session).
	// Antigravity cannot refresh "gemini"-bound tokens (unauthorized_client),
	// so switching the IDE to such an account ends the session within ~1h.
	Client string `json:"client,omitempty"`
}

func (o *OAuthData) IsExpired() bool {
	if o == nil || o.ExpiryDate <= 0 {
		return false
	}
	// Expired if current time ms is within 60s of expiry
	return time.Now().UnixMilli() >= (o.ExpiryDate - 60000)
}

type QuotaBucket struct {
	BucketID          string  `json:"bucket_id,omitempty"`
	ModelID           string  `json:"model_id,omitempty"`
	DisplayName       string  `json:"display_name,omitempty"`
	Description       string  `json:"description,omitempty"`
	Window            string  `json:"window,omitempty"`
	RemainingAmount   int     `json:"remaining_amount,omitempty"`
	RemainingFraction float64 `json:"remaining_fraction"`
	Limit             int     `json:"limit,omitempty"`
	ResetTime         string  `json:"reset_time,omitempty"`
}

type QuotaGroup struct {
	DisplayName string        `json:"display_name"`
	Description string        `json:"description,omitempty"`
	Buckets     []QuotaBucket `json:"buckets"`
}

type QuotaInfo struct {
	UpdatedAt   time.Time     `json:"updated_at"`
	Tier        string        `json:"tier,omitempty"`
	Groups      []QuotaGroup  `json:"groups,omitempty"`
	Buckets     []QuotaBucket `json:"buckets,omitempty"`
	Description string        `json:"description,omitempty"`
}

type Account struct {
	ID        string      `json:"id"`
	Type      AccountType `json:"type"`
	Name      string      `json:"name"`
	Email     string      `json:"email,omitempty"`
	Picture   string      `json:"picture,omitempty"`
	ProjectID string      `json:"project_id,omitempty"`
	OAuth     *OAuthData  `json:"oauth,omitempty"`
	APIKey    string      `json:"api_key,omitempty"`
	RPMLimit  int         `json:"rpm_limit,omitempty"`
	RPDLimit  int         `json:"rpd_limit,omitempty"`
	LastQuota *QuotaInfo  `json:"last_quota,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type StoreData struct {
	Version         int        `json:"version"`
	ActiveAccountID string     `json:"active_account_id"`
	Accounts        []*Account `json:"accounts"`
	ProxyPort       int        `json:"proxy_port"`
	AutoRotate      bool       `json:"auto_rotate"`
	LastUpdated     time.Time  `json:"last_updated"`
}
