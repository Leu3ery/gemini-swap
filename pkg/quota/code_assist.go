package quota

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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

func findAntigravityServer() (int, string, error) {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		return 0, "", err
	}

	var pid string
	var csrfToken string

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
		return 0, "", fmt.Errorf("antigravity language_server not running")
	}

	// Try default/common port 61955 first
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
		return 61955, csrfToken, nil
	}

	// If not 61955, check open listening ports for this PID
	lsofOut, lErr := exec.Command("lsof", "-nP", "-p", pid).Output()
	if lErr == nil {
		rePort := regexp.MustCompile(`:(\d+)\s+\(LISTEN\)`)
		for _, match := range rePort.FindAllStringSubmatch(string(lsofOut), -1) {
			if len(match) >= 2 {
				p, _ := strconv.Atoi(match[1])
				if p > 0 && testPort(p) {
					return p, csrfToken, nil
				}
			}
		}
	}

	return 0, "", fmt.Errorf("could not connect to antigravity language server")
}

func fetchAntigravityQuota() (*account.QuotaInfo, error) {
	port, csrf, err := findAntigravityServer()
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

func buildDefaultAntigravityQuota() *account.QuotaInfo {
	return &account.QuotaInfo{
		UpdatedAt:   time.Now(),
		Tier:        "Google AI Pro",
		Description: "Within each group, models share a weekly limit and a 5-hour limit.",
		Groups: []account.QuotaGroup{
			{
				DisplayName: "Gemini Models",
				Description: "Models within this group: Gemini Flash, Gemini Pro",
				Buckets: []account.QuotaBucket{
					{
						BucketID:          "gemini-weekly",
						DisplayName:       "Weekly Limit Remaining",
						Description:       "You have used some of your weekly limit, it will fully refresh in a few days.",
						Window:            "weekly",
						RemainingFraction: 1.0,
						ResetTime:         time.Now().Add(5 * 24 * time.Hour).Format(time.RFC3339),
					},
					{
						BucketID:          "gemini-5h",
						DisplayName:       "Five Hour Limit Remaining",
						Description:       "You have used some of your 5-hour limit, it will fully refresh in 5 hours.",
						Window:            "5h",
						RemainingFraction: 1.0,
						ResetTime:         time.Now().Add(5 * time.Hour).Format(time.RFC3339),
					},
				},
			},
			{
				DisplayName: "Claude and GPT models",
				Description: "Models within this group: Claude Opus, Claude Sonnet, GPT-OSS",
				Buckets: []account.QuotaBucket{
					{
						BucketID:          "3p-weekly",
						DisplayName:       "Weekly Limit Remaining",
						Window:            "weekly",
						RemainingFraction: 1.0,
						ResetTime:         time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
					},
					{
						BucketID:          "3p-5h",
						DisplayName:       "Five Hour Limit Remaining",
						Window:            "5h",
						RemainingFraction: 1.0,
						ResetTime:         time.Now().Add(5 * time.Hour).Format(time.RFC3339),
					},
				},
			},
		},
	}
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
		_ = auth.RefreshToken(acc.OAuth)
	}

	// 1. Try fetching live Antigravity limits from local language server
	if q, err := fetchAntigravityQuota(); err == nil && len(q.Groups) > 0 {
		acc.LastQuota = q
		return q, nil
	}

	// 2. If account already has last quota with groups, update timestamp and return
	if acc.LastQuota != nil && len(acc.LastQuota.Groups) > 0 {
		acc.LastQuota.UpdatedAt = time.Now()
		return acc.LastQuota, nil
	}

	// 3. Fallback to default Antigravity quota
	q := buildDefaultAntigravityQuota()
	acc.LastQuota = q
	return q, nil
}
