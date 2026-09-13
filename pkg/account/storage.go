package account

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	DirName        = ".gemini-swap"
	StoreFileName  = "accounts.json"
	DefaultPort    = 8045
	CurrentVersion = 1
)

type Storage struct {
	baseDir string
	mu      sync.Mutex
}

func GetDefaultBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DirName), nil
}

func NewStorage() (*Storage, error) {
	baseDir, err := GetDefaultBaseDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory %s: %w", baseDir, err)
	}
	s := &Storage{baseDir: baseDir}
	// Try initial auto-migration if no accounts exist
	_ = s.autoImportExisting()
	return s, nil
}

func (s *Storage) GetFilePath() string {
	return filepath.Join(s.baseDir, StoreFileName)
}

func (s *Storage) Load() (*StoreData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.GetFilePath()
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		initial := &StoreData{
			Version:         CurrentVersion,
			Accounts:        make([]*Account, 0),
			ProxyPort:       DefaultPort,
			AutoRotate:      true,
			LastUpdated:     time.Now(),
		}
		return initial, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	var store StoreData
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}
	if store.ProxyPort <= 0 {
		store.ProxyPort = DefaultPort
	}
	return &store, nil
}

func (s *Storage) Save(store *StoreData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	store.LastUpdated = time.Now()
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	filePath := s.GetFilePath()
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, filePath)
}

func (s *Storage) GetActiveAccount() (*Account, error) {
	store, err := s.Load()
	if err != nil {
		return nil, err
	}
	if store.ActiveAccountID == "" && len(store.Accounts) > 0 {
		return store.Accounts[0], nil
	}
	for _, acc := range store.Accounts {
		if acc.ID == store.ActiveAccountID {
			return acc, nil
		}
	}
	if len(store.Accounts) > 0 {
		return store.Accounts[0], nil
	}
	return nil, fmt.Errorf("no accounts configured")
}

func (s *Storage) SetActiveAccount(idOrEmail string) (*Account, error) {
	store, err := s.Load()
	if err != nil {
		return nil, err
	}

	var target *Account
	for _, acc := range store.Accounts {
		if acc.ID == idOrEmail || acc.Email == idOrEmail || acc.Name == idOrEmail {
			target = acc
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("account not found: %s", idOrEmail)
	}

	store.ActiveAccountID = target.ID
	if err := s.Save(store); err != nil {
		return nil, err
	}

	// Synchronize to ~/.gemini/
	if err := SyncToGeminiCLI(target); err != nil {
		// Non-fatal warning
		fmt.Fprintf(os.Stderr, "Warning: failed to sync with ~/.gemini: %v\n", err)
	}

	// Synchronize to Antigravity
	if err := SyncToAntigravity(target); err != nil {
		// Non-fatal warning
		fmt.Fprintf(os.Stderr, "Warning: failed to sync with Antigravity: %v\n", err)
	}

	return target, nil
}

func (s *Storage) AddAccount(acc *Account) error {
	store, err := s.Load()
	if err != nil {
		return err
	}

	// Update existing if ID or email already exists
	updated := false
	for i, existing := range store.Accounts {
		if existing.ID == acc.ID || (acc.Email != "" && existing.Email == acc.Email) {
			acc.ID = existing.ID
			acc.CreatedAt = existing.CreatedAt
			acc.UpdatedAt = time.Now()
			store.Accounts[i] = acc
			updated = true
			break
		}
	}

	if !updated {
		acc.CreatedAt = time.Now()
		acc.UpdatedAt = time.Now()
		store.Accounts = append(store.Accounts, acc)
	}

	// If no active account, make this active
	if store.ActiveAccountID == "" {
		store.ActiveAccountID = acc.ID
	}

	if err := s.Save(store); err != nil {
		return err
	}

	if store.ActiveAccountID == acc.ID {
		_ = SyncToGeminiCLI(acc)
	}

	return nil
}

func (s *Storage) RemoveAccount(idOrEmail string) error {
	store, err := s.Load()
	if err != nil {
		return err
	}

	newAccounts := make([]*Account, 0, len(store.Accounts))
	var removed bool
	for _, acc := range store.Accounts {
		if acc.ID == idOrEmail || acc.Email == idOrEmail || acc.Name == idOrEmail {
			removed = true
			continue
		}
		newAccounts = append(newAccounts, acc)
	}

	if !removed {
		return fmt.Errorf("account '%s' not found", idOrEmail)
	}

	store.Accounts = newAccounts
	if store.ActiveAccountID == idOrEmail || len(store.Accounts) > 0 && (store.ActiveAccountID == "") {
		if len(store.Accounts) > 0 {
			store.ActiveAccountID = store.Accounts[0].ID
			_ = SyncToGeminiCLI(store.Accounts[0])
		} else {
			store.ActiveAccountID = ""
		}
	}

	return s.Save(store)
}

// ExportPayload represents the portable export format
type ExportPayload struct {
	GeminiSwapExport int        `json:"gemini_swap_export"`
	ExportedAt       time.Time  `json:"exported_at"`
	Account          *Account   `json:"account"`
}

func (s *Storage) ExportAccount(idOrEmail string, outPath string) (string, string, error) {
	store, err := s.Load()
	if err != nil {
		return "", "", err
	}

	var target *Account
	for _, acc := range store.Accounts {
		if acc.ID == idOrEmail || acc.Email == idOrEmail || acc.Name == idOrEmail {
			target = acc
			break
		}
	}
	if target == nil {
		return "", "", fmt.Errorf("account not found: %s", idOrEmail)
	}

	payload := ExportPayload{
		GeminiSwapExport: 1,
		ExportedAt:       time.Now(),
		Account:          target,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", "", err
	}

	// Determine output file path if not provided
	if outPath == "" {
		safeName := target.Name
		if safeName == "" {
			safeName = target.Email
		}
		if safeName == "" {
			safeName = target.ID
		}
		// sanitize
		safeName = filepath.Base(safeName)
		outPath = fmt.Sprintf("gemini-account-%s.json", safeName)
	}

	if err := os.WriteFile(outPath, data, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write export file %s: %w", outPath, err)
	}

	// Also generate compact base64 share string
	shareCode := "gswap_" + base64StdEncode(data)

	return shareCode, outPath, nil
}

func (s *Storage) ImportAccount(source string) (*Account, error) {
	var rawData []byte

	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "gswap_") {
		// Base64 share code
		encoded := strings.TrimPrefix(trimmed, "gswap_")
		decoded, err := base64StdDecode(encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid share code: %w", err)
		}
		rawData = decoded
	} else if _, err := os.Stat(trimmed); err == nil {
		// File path
		data, err := os.ReadFile(trimmed)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", trimmed, err)
		}
		rawData = data
	} else {
		// Try raw JSON string or invalid path
		rawData = []byte(trimmed)
	}

	// Try unmarshaling as ExportPayload
	var payload ExportPayload
	if err := json.Unmarshal(rawData, &payload); err == nil && payload.Account != nil {
		acc := payload.Account
		if acc.ID == "" {
			acc.ID = fmt.Sprintf("%s-imported-%d", acc.Type, time.Now().Unix())
		}
		acc.UpdatedAt = time.Now()
		if err := s.AddAccount(acc); err != nil {
			return nil, err
		}
		return acc, nil
	}

	// Try unmarshaling as raw Account
	var rawAcc Account
	if err := json.Unmarshal(rawData, &rawAcc); err == nil && (rawAcc.Type != "" || rawAcc.APIKey != "" || rawAcc.OAuth != nil) {
		if rawAcc.ID == "" {
			rawAcc.ID = fmt.Sprintf("imported-%d", time.Now().Unix())
		}
		rawAcc.UpdatedAt = time.Now()
		if err := s.AddAccount(&rawAcc); err != nil {
			return nil, err
		}
		return &rawAcc, nil
	}

	return nil, fmt.Errorf("failed to parse account data: unrecognized format")
}

func base64StdEncode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func base64StdDecode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
