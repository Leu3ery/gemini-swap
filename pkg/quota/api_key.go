package quota

import (
		"fmt"
	"io"
	"net/http"
	"time"

	"gemini-swap/pkg/account"
)

const GenerativeLanguageAPI = "https://generativelanguage.googleapis.com/v1beta"

type ListModelsResponse struct {
	Models []struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
	} `json:"models"`
}

func ValidateAPIKey(apiKey string) error {
	url := fmt.Sprintf("%s/models?key=%s", GenerativeLanguageAPI, apiKey)
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api key validation failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func FetchAPIKeyQuota(acc *account.Account) (*account.QuotaInfo, error) {
	if acc.APIKey == "" {
		return nil, fmt.Errorf("no api key specified for account %s", acc.Name)
	}

	_ = ValidateAPIKey(acc.APIKey)

	rpm := acc.RPMLimit
	if rpm <= 0 {
		rpm = 15 // Default Google AI Studio Free Tier
	}
	rpd := acc.RPDLimit
	if rpd <= 0 {
		rpd = 1500
	}

	quotaInfo := &account.QuotaInfo{
		UpdatedAt: time.Now(),
		Tier:      "AI Studio API Key",
		Buckets: []account.QuotaBucket{
			{
				ModelID:           "gemini-2.5-flash",
				RemainingAmount:   rpd,
				RemainingFraction: 1.0,
				Limit:             rpd,
				ResetTime:         time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).Format(time.RFC3339),
			},
			{
				ModelID:           "gemini-2.5-pro",
				RemainingAmount:   50,
				RemainingFraction: 1.0,
				Limit:             50,
				ResetTime:         time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	acc.LastQuota = quotaInfo
	return quotaInfo, nil
}
