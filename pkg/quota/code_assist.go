package quota

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gemini-swap/pkg/account"
	"gemini-swap/pkg/auth"
)

type AntigravityQuotaResponse struct {
	Response struct {
		Groups []struct {
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
			Buckets     []struct {
				BucketID          string  `json:"bucketId"`
				DisplayName       string  `json:"displayName"`
				Description       string  `json:"description"`
				Window            string  `json:"window"`
				RemainingFraction float64 `json:"remainingFraction"`
				ResetTime         string  `json:"resetTime"`
			} `json:"buckets"`
		} `json:"groups"`
		Description string `json:"description"`
	} `json:"response"`
}

type AntigravityUserStatusResponse struct {
	UserTier struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"userTier"`
}

func fetchAntigravityQuota() (*account.QuotaInfo, error) {
	port, csrf, _, err := account.FindAntigravityServer()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Retrieve user quota summary
	quotaURL := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary", port)
	req, err := http.NewRequest("POST", quotaURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-codeium-csrf-token", csrf)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var quotaResp AntigravityQuotaResponse
	if err := json.Unmarshal(body, &quotaResp); err != nil {
		return nil, err
	}

	// Also fetch user tier if possible
	tierName := "Google AI Pro"
	statusURL := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/GetUserStatus", port)
	sReq, sErr := http.NewRequest("POST", statusURL, bytes.NewReader([]byte("{}")))
	if sErr == nil {
		sReq.Header.Set("x-codeium-csrf-token", csrf)
		sReq.Header.Set("Content-Type", "application/json")
		sResp, doErr := client.Do(sReq)
		if doErr == nil {
			defer sResp.Body.Close()
			sBody, _ := io.ReadAll(sResp.Body)
			var statusResp AntigravityUserStatusResponse
			if json.Unmarshal(sBody, &statusResp) == nil && statusResp.UserTier.Name != "" {
				tierName = statusResp.UserTier.Name
			}
		}
	}

	quotaInfo := &account.QuotaInfo{
		UpdatedAt:   time.Now(),
		Tier:        tierName,
		Description: quotaResp.Response.Description,
		Groups:      make([]account.QuotaGroup, 0),
		Buckets:     make([]account.QuotaBucket, 0),
	}

	for _, g := range quotaResp.Response.Groups {
		group := account.QuotaGroup{
			DisplayName: g.DisplayName,
			Description: g.Description,
			Buckets:     make([]account.QuotaBucket, 0),
		}
		for _, b := range g.Buckets {
			bucket := account.QuotaBucket{
				BucketID:          b.BucketID,
				DisplayName:       b.DisplayName,
				Description:       b.Description,
				Window:            b.Window,
				RemainingFraction: b.RemainingFraction,
				ResetTime:         b.ResetTime,
			}
			group.Buckets = append(group.Buckets, bucket)
			quotaInfo.Buckets = append(quotaInfo.Buckets, bucket)
		}
		quotaInfo.Groups = append(quotaInfo.Groups, group)
	}

	return quotaInfo, nil
}

func FetchAccountQuota(acc *account.Account, isActive bool) (*account.QuotaInfo, error) {
	if acc.Type == account.TypeAPIKey {
		return FetchAPIKeyQuota(acc)
	}

	if acc.OAuth == nil {
		return nil, fmt.Errorf("no oauth credentials for account %s", acc.Name)
	}

	// Check expiry and refresh if needed
	if acc.OAuth.IsExpired() {
		_ = auth.RefreshToken(acc.OAuth)
	}

	// 1. Live limits come from the local Antigravity language server, which
	// always reflects the IDE's CURRENT session — so they may only be
	// attributed to the matching account. Attributing them to any other
	// account is how two cards ended up showing identical numbers.
	if isActive {
		if q, err := fetchAntigravityQuota(); err == nil && len(q.Groups) > 0 {
			acc.LastQuota = q
			return q, nil
		}
	}

	// 2. Otherwise keep this account's own last-known quota (frozen in time).
	if acc.LastQuota != nil && (len(acc.LastQuota.Groups) > 0 || len(acc.LastQuota.Buckets) > 0) {
		return acc.LastQuota, nil
	}

	return nil, fmt.Errorf("no quota data for %s — switch to it and refresh", acc.Name)
}

// SessionAccountID matches the live IDE session (keychain token) against the
// store without any network calls, by comparing refresh/access tokens.
func SessionAccountID(store *account.StoreData) string {
	if store == nil {
		return ""
	}
	sess, err := account.ReadAntigravitySessionToken()
	if err != nil || sess == nil {
		return ""
	}
	for _, a := range store.Accounts {
		if a.Type != account.TypeOAuth || a.OAuth == nil {
			continue
		}
		if sess.RefreshToken != "" && a.OAuth.RefreshToken == sess.RefreshToken {
			return a.ID
		}
		if sess.AccessToken != "" && a.OAuth.AccessToken == sess.AccessToken {
			return a.ID
		}
	}
	return ""
}
