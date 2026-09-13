package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
		"sync"
	"time"

	"gemini-swap/pkg/account"
	"gemini-swap/pkg/auth"
)

type RequestLog struct {
	Timestamp   time.Time `json:"timestamp"`
	AccountID   string    `json:"account_id"`
	AccountName string    `json:"account_name"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	StatusCode  int       `json:"status_code"`
	DurationMs  int64     `json:"duration_ms"`
	Rotated     bool      `json:"rotated,omitempty"`
}

type ProxyStats struct {
	TotalRequests int          `json:"total_requests"`
	Rotations     int          `json:"rotations"`
	ActiveAccount string       `json:"active_account"`
	RecentLogs    []RequestLog `json:"recent_logs"`
	LastUpdated   time.Time    `json:"last_updated"`
}

type Server struct {
	storage *account.Storage
	port    int
	stats   ProxyStats
	mu      sync.Mutex
	server  *http.Server
}

func NewServer(storage *account.Storage, port int) *Server {
	return &Server{
		storage: storage,
		port:    port,
		stats: ProxyStats{
			RecentLogs: make([]RequestLog, 0),
		},
	}
}

func (s *Server) saveStats() {
	s.mu.Lock()
	defer s.mu.Unlock()

	baseDir, err := account.GetDefaultBaseDir()
	if err != nil {
		return
	}

	s.stats.LastUpdated = time.Now()
	data, _ := json.MarshalIndent(s.stats, "", "  ")
	_ = os.WriteFile(filepath.Join(baseDir, "stats.json"), data, 0644)
}

func (s *Server) recordLog(l RequestLog) {
	s.mu.Lock()
	s.stats.TotalRequests++
	if l.Rotated {
		s.stats.Rotations++
	}
	s.stats.ActiveAccount = l.AccountName
	if len(s.stats.RecentLogs) >= 50 {
		s.stats.RecentLogs = s.stats.RecentLogs[1:]
	}
	s.stats.RecentLogs = append(s.stats.RecentLogs, l)
	s.mu.Unlock()

	s.saveStats()
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Stats endpoint for GUI / CLI
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.stats)
	})

	// Switch account endpoint for GUI / CLI
	mux.HandleFunc("/api/switch", func(w http.ResponseWriter, r *http.Request) {
		accID := r.URL.Query().Get("id")
		if accID == "" {
			http.Error(w, "missing id query param", http.StatusBadRequest)
			return
		}
		target, err := s.storage.SetActiveAccount(accID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(target)
	})

	// Forward all proxy requests (Gemini or OpenAI format)
	mux.HandleFunc("/", s.handleProxy)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler: mux,
	}

	log.Printf("[Proxy] Gemini Swap proxy running on http://127.0.0.1:%d", s.port)

	go func() {
		<-ctx.Done()
		_ = s.server.Shutdown(context.Background())
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	bodyBytes, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	store, err := s.storage.Load()
	if err != nil || len(store.Accounts) == 0 {
		http.Error(w, "No Gemini accounts configured in Gemini Swap", http.StatusBadGateway)
		return
	}

	currentAcc, err := s.storage.GetActiveAccount()
	if err != nil {
		http.Error(w, "No active account found", http.StatusBadGateway)
		return
	}

	// Try request, with auto-rotation if 429
	rotated := false
	maxAttempts := 1
	if store.AutoRotate && len(store.Accounts) > 1 {
		maxAttempts = len(store.Accounts)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		respCode, err := s.forwardRequest(w, r, bytes.NewReader(bodyBytes), currentAcc)
		if err == nil && respCode != http.StatusTooManyRequests {
			s.recordLog(RequestLog{
				Timestamp:   time.Now(),
				AccountID:   currentAcc.ID,
				AccountName: currentAcc.Name,
				Path:        r.URL.Path,
				Method:      r.Method,
				StatusCode:  respCode,
				DurationMs:  time.Since(start).Milliseconds(),
				Rotated:     rotated,
			})
			return
		}

		if respCode == http.StatusTooManyRequests && store.AutoRotate && attempt < maxAttempts-1 {
			log.Printf("[Proxy] Account '%s' hit 429 rate limit. Rotating to next account...", currentAcc.Name)
			rotated = true
			// Find next account
			nextAcc := s.findNextAccount(store, currentAcc.ID)
			if nextAcc != nil {
				_, _ = s.storage.SetActiveAccount(nextAcc.ID)
				currentAcc = nextAcc
				continue
			}
		}

		// Non-retryable error or exhausted accounts
		s.recordLog(RequestLog{
			Timestamp:   time.Now(),
			AccountID:   currentAcc.ID,
			AccountName: currentAcc.Name,
			Path:        r.URL.Path,
			Method:      r.Method,
			StatusCode:  respCode,
			DurationMs:  time.Since(start).Milliseconds(),
			Rotated:     rotated,
		})
		return
	}
}

func (s *Server) findNextAccount(store *account.StoreData, currentID string) *account.Account {
	idx := -1
	for i, a := range store.Accounts {
		if a.ID == currentID {
			idx = i
			break
		}
	}
	if idx == -1 || len(store.Accounts) <= 1 {
		return nil
	}
	nextIdx := (idx + 1) % len(store.Accounts)
	return store.Accounts[nextIdx]
}

func (s *Server) forwardRequest(w http.ResponseWriter, r *http.Request, bodyReader io.Reader, acc *account.Account) (int, error) {
	targetURL, _ := url.Parse("https://generativelanguage.googleapis.com")

	// If OAuth, ensure refreshed
	if acc.Type == account.TypeOAuth && acc.OAuth != nil {
		if acc.OAuth.IsExpired() {
			_ = auth.RefreshToken(acc.OAuth)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom Director to inject credentials
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = targetURL.Host
		if acc.Type == account.TypeAPIKey && acc.APIKey != "" {
			q := req.URL.Query()
			q.Set("key", acc.APIKey)
			req.URL.RawQuery = q.Encode()
		} else if acc.Type == account.TypeOAuth && acc.OAuth != nil {
			req.Header.Set("Authorization", "Bearer "+acc.OAuth.AccessToken)
		}
	}

	// Capture response code for rotation check
	recorder := &responseCaptureWriter{ResponseWriter: w, statusCode: http.StatusOK}
	proxy.ServeHTTP(recorder, r)

	return recorder.statusCode, nil
}

type responseCaptureWriter struct {
	http.ResponseWriter
	statusCode int
	wroteHead  bool
}

func (rc *responseCaptureWriter) WriteHeader(code int) {
	rc.statusCode = code
	if !rc.wroteHead {
		rc.ResponseWriter.WriteHeader(code)
		rc.wroteHead = true
	}
}

func (rc *responseCaptureWriter) Write(b []byte) (int, error) {
	if !rc.wroteHead {
		rc.WriteHeader(http.StatusOK)
	}
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCaptureWriter) Flush() {
	if f, ok := rc.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
