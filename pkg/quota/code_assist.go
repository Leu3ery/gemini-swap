package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"gemini-swap/pkg/account"
	"gemini-swap/pkg/auth"
)

const (
	CodeAssistEndpoint = "https://cloudcode-pa.googleapis.com/v1internal"
)

type LoadCodeAssistRequest struct {
	CloudaicompanionProject string                 `json:"cloudaicompanionProject,omitempty"`
	Metadata                map[string]interface{} `json:"metadata"`
}

type TierInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LoadCodeAssistResponse struct {
	CloudaicompanionProject string    `json:"cloudaicompanionProject"`
	CurrentTier             *TierInfo `json:"currentTier"`
	PaidTier                *TierInfo `json:"paidTier"`
	IneligibleTiers         []struct {
		ReasonMessage string `json:"reasonMessage"`
		TierName      string `json:"tierName"`
	} `json:"ineligibleTiers"`
}

type RetrieveUserQuotaRequest struct {
	Project string `json:"project"`
}

type RawBucket struct {
	ModelID           string  `json:"modelId"`
	RemainingAmount   string  `json:"remainingAmount"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

type RetrieveUserQuotaResponse struct {
	Buckets []RawBucket `json:"buckets"`
}

func FetchAccountQuota(acc *account.Account) (*account.QuotaInfo, error) {
	if acc.Type == account.TypeAPIKey {
		return FetchAPIKeyQuota(acc)
	}

	if acc.OAuth == nil {
		return nil, fmt.Errorf("no oauth credentials for account %s", acc.Name)
	}

	// Check expiry and refresh if needed
	if acc.OAuth.IsExpired() {
		if err := auth.RefreshToken(acc.OAuth); err != nil {
			return nil, fmt.Errorf("failed to refresh oauth token: %w", err)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Step 1: loadCodeAssist
	loadReqBody, _ := json.Marshal(LoadCodeAssistRequest{
		CloudaicompanionProject: acc.ProjectID,
		Metadata: map[string]interface{}{
			"ideType":     "IDE_UNSPECIFIED",
			"platform":    "PLATFORM_UNSPECIFIED",
			"pluginType":  "GEMINI",
			"duetProject": acc.ProjectID,
		},
	})

	req, err := http.NewRequest("POST", CodeAssistEndpoint+":loadCodeAssist", bytes.NewReader(loadReqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+acc.OAuth.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loadCodeAssist request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loadCodeAssist returned status %d: %s", resp.StatusCode, string(body))
	}

	var loadResp LoadCodeAssistResponse
	if err := json.Unmarshal(body, &loadResp); err != nil {
		return nil, fmt.Errorf("failed to decode loadCodeAssist response: %w", err)
	}

	projectID := acc.ProjectID
	if loadResp.CloudaicompanionProject != "" {
		projectID = loadResp.CloudaicompanionProject
		acc.ProjectID = projectID
	}

	tierName := "Gemini Code Assist"
	if loadResp.CurrentTier != nil && loadResp.CurrentTier.Name != "" {
		tierName = loadResp.CurrentTier.Name
	} else if loadResp.PaidTier != nil && loadResp.PaidTier.Name != "" {
		tierName = loadResp.PaidTier.Name
	}

	quotaInfo := &account.QuotaInfo{
		UpdatedAt: time.Now(),
		Tier:      tierName,
		Buckets:   make([]account.QuotaBucket, 0),
	}

	// Step 2: retrieveUserQuota if projectID exists
	if projectID != "" {
		quotaReqBody, _ := json.Marshal(RetrieveUserQuotaRequest{
			Project: projectID,
		})

		qReq, err := http.NewRequest("POST", CodeAssistEndpoint+":retrieveUserQuota", bytes.NewReader(quotaReqBody))
		if err == nil {
			qReq.Header.Set("Authorization", "Bearer "+acc.OAuth.AccessToken)
			qReq.Header.Set("Content-Type", "application/json")

			qResp, err := client.Do(qReq)
			if err == nil {
				defer qResp.Body.Close()
				qBody, _ := io.ReadAll(qResp.Body)
				if qResp.StatusCode == http.StatusOK {
					var uQuota RetrieveUserQuotaResponse
					if json.Unmarshal(qBody, &uQuota) == nil {
						for _, b := range uQuota.Buckets {
							remAmt, _ := strconv.Atoi(b.RemainingAmount)
							limit := 100
							if remAmt > 0 && b.RemainingFraction > 0 {
								limit = int(float64(remAmt) / b.RemainingFraction)
							}
							quotaInfo.Buckets = append(quotaInfo.Buckets, account.QuotaBucket{
								ModelID:           b.ModelID,
								RemainingAmount:   remAmt,
								RemainingFraction: b.RemainingFraction,
								Limit:             limit,
								ResetTime:         b.ResetTime,
							})
						}
					}
				}
			}
		}
	}

	// If no buckets returned from API, provide default tier estimation
	if len(quotaInfo.Buckets) == 0 {
		quotaInfo.Buckets = append(quotaInfo.Buckets, account.QuotaBucket{
			ModelID:           "gemini-2.5-flash",
			RemainingAmount:   1000,
			RemainingFraction: 1.0,
			Limit:             1000,
			ResetTime:         time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		})
		quotaInfo.Buckets = append(quotaInfo.Buckets, account.QuotaBucket{
			ModelID:           "gemini-2.5-pro",
			RemainingAmount:   50,
			RemainingFraction: 1.0,
			Limit:             50,
			ResetTime:         time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		})
	}

	acc.LastQuota = quotaInfo
	return quotaInfo, nil
}
