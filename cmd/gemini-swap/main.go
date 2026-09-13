package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gemini-swap/pkg/account"
	"gemini-swap/pkg/auth"
	"gemini-swap/pkg/proxy"
	"gemini-swap/pkg/quota"
	"gemini-swap/pkg/web"
)

const Version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	storage, err := account.NewStorage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "list", "ls", "accounts":
		handleList(storage, args)
	case "switch", "use":
		handleSwitch(storage, args)
	case "current", "whoami":
		handleCurrent(storage, args)
	case "quota", "usage":
		handleQuota(storage, args)
	case "login", "auth":
		handleLogin(storage, args)
	case "add-key":
		handleAddKey(storage, args)
	case "remove", "rm", "delete":
		handleRemove(storage, args)
	case "export", "share":
		handleExport(storage, args)
	case "import", "load":
		handleImport(storage, args)
	case "exec", "run":
		handleExec(storage, args)
	case "proxy":
		handleProxy(storage, args)
	case "web":
		handleWeb(storage, args)
	case "version", "-v", "--version":
		fmt.Printf("gemini-swap version %s\n", Version)
	case "help", "-h", "--help":
		printUsage()
	default:
		// Check if the command matches an account name/email to switch directly
		acc, sErr := storage.SetActiveAccount(command)
		if sErr == nil {
			fmt.Printf("✓ Switched active account to: %s (%s)\n", acc.Name, acc.Email)
			return
		}
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'gemini-swap help' for usage.\n", command)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Gemini Swap - Multi-Account Switcher & Quota Monitor for Gemini

USAGE:
  gemini-swap <command> [arguments]

CORE COMMANDS:
  list, ls             List all accounts, remaining quotas, and active status
  switch <id|email>    Switch the active Gemini account seamlessly
  current              Show details of currently active account
  quota [id|email]     Fetch latest quota and rate limits from Google
  login [--headless]   Log in to a new Google account via OAuth
  add-key              Add a Gemini AI Studio API key
  remove <id|email>    Remove an account
  export <id|email>    Export account config to a shareable file or code
  import <file|code>   Import account config from a friend

INTEGRATION & TOOLS:
  exec -- <cmd...>     Run a command with active Gemini credentials injected
  proxy [--port 8045]  Start local proxy with auto-failover/rotation for Codex
  web [--port 8080]    Start web dashboard (ideal for VPS)
  version              Show version info

EXAMPLES:
  gemini-swap list
  gemini-swap switch user@gmail.com
  gemini-swap exec -- codex
  eval $(gemini-swap current --env)`)
}

func handleList(storage *account.Storage, args []string) {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		}
	}

	store, err := storage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if asJSON {
		b, _ := json.MarshalIndent(store, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts found. Add one with 'gemini-swap login' or 'gemini-swap add-key'.")
		return
	}

	fmt.Printf("\n%-3s %-24s %-10s %-32s %-20s\n", "ACT", "NAME", "TYPE", "EMAIL/ID", "QUOTA (FLASH / PRO)")
	fmt.Println(strings.Repeat("-", 94))

	for _, acc := range store.Accounts {
		activeMark := " "
		if acc.ID == store.ActiveAccountID {
			activeMark = "★"
		}

		typeStr := string(acc.Type)
		emailOrID := acc.Email
		if emailOrID == "" {
			emailOrID = acc.ID
		}

		quotaSummary := "No quota data"
		if acc.LastQuota != nil && len(acc.LastQuota.Buckets) > 0 {
			var parts []string
			for _, b := range acc.LastQuota.Buckets {
				pct := int(b.RemainingFraction * 100)
				shortName := b.ModelID
				if strings.Contains(shortName, "flash") {
					shortName = "Flash"
				} else if strings.Contains(shortName, "pro") {
					shortName = "Pro"
				}
				parts = append(parts, fmt.Sprintf("%s: %d%%", shortName, pct))
			}
			quotaSummary = strings.Join(parts, " | ")
		}

		name := acc.Name
		if len(name) > 22 {
			name = name[:19] + "..."
		}
		if len(emailOrID) > 30 {
			emailOrID = emailOrID[:27] + "..."
		}

		fmt.Printf("%-3s %-24s %-10s %-32s %-20s\n", activeMark, name, typeStr, emailOrID, quotaSummary)
	}
	fmt.Println()
}

func handleSwitch(storage *account.Storage, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gemini-swap switch <account-id | email | name>")
		os.Exit(1)
	}

	target := args[0]
	acc, err := storage.SetActiveAccount(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Active account switched to: %s (%s)\n", acc.Name, acc.Email)
	fmt.Println("  Synchronized ~/.gemini/oauth_creds.json and ~/.gemini/google_accounts.json")
	if acc.Type == account.TypeOAuth {
		fmt.Println("  Synchronized Google Antigravity session & credentials")
		// Refresh quota for the newly active account
		if q, qErr := quota.FetchAccountQuota(acc, true); qErr == nil && q != nil {
			acc.LastQuota = q
			_ = storage.AddAccount(acc)
		}
	}
}

func handleCurrent(storage *account.Storage, args []string) {
	envMode := false
	jsonMode := false
	tokenOnly := false
	keyOnly := false

	for _, a := range args {
		switch a {
		case "--env":
			envMode = true
		case "--json":
			jsonMode = true
		case "--token":
			tokenOnly = true
		case "--key":
			keyOnly = true
		}
	}

	acc, err := storage.GetActiveAccount()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if tokenOnly {
		if acc.OAuth != nil {
			if acc.OAuth.IsExpired() {
				_ = auth.RefreshToken(acc.OAuth)
				_ = storage.AddAccount(acc)
			}
			fmt.Println(acc.OAuth.AccessToken)
		}
		return
	}

	if keyOnly {
		fmt.Println(acc.APIKey)
		return
	}

	if jsonMode {
		b, _ := json.MarshalIndent(acc, "", "  ")
		fmt.Println(string(b))
		return
	}

	if envMode {
		if acc.Type == account.TypeAPIKey {
			fmt.Printf("export GEMINI_API_KEY=\"%s\"\nexport GOOGLE_API_KEY=\"%s\"\n", acc.APIKey, acc.APIKey)
		} else if acc.OAuth != nil {
			if acc.OAuth.IsExpired() {
				_ = auth.RefreshToken(acc.OAuth)
				_ = storage.AddAccount(acc)
			}
			fmt.Printf("export GOOGLE_GENAI_USE_GCA=true\nexport GOOGLE_CLOUD_ACCESS_TOKEN=\"%s\"\n", acc.OAuth.AccessToken)
		}
		return
	}

	fmt.Printf("\nActive Account: %s\n", acc.Name)
	fmt.Printf("  ID:      %s\n", acc.ID)
	fmt.Printf("  Type:    %s\n", acc.Type)
	if acc.Email != "" {
		fmt.Printf("  Email:   %s\n", acc.Email)
	}
	if acc.ProjectID != "" {
		fmt.Printf("  Project: %s\n", acc.ProjectID)
	}
	if acc.LastQuota != nil {
		fmt.Printf("  Tier:    %s (updated %s)\n", acc.LastQuota.Tier, acc.LastQuota.UpdatedAt.Format("15:04:05"))
	}
	fmt.Println()
}

func handleQuota(storage *account.Storage, args []string) {
	store, err := storage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var targets []*account.Account
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		idOrEmail := args[0]
		for _, a := range store.Accounts {
			if a.ID == idOrEmail || a.Email == idOrEmail || a.Name == idOrEmail {
				targets = append(targets, a)
				break
			}
		}
		if len(targets) == 0 {
			fmt.Fprintf(os.Stderr, "Account '%s' not found\n", idOrEmail)
			os.Exit(1)
		}
	} else {
		targets = store.Accounts
	}

	fmt.Println("\nFetching latest usage & quotas from Google...")
	for _, acc := range targets {
		fmt.Printf("\n=== %s (%s) ===\n", acc.Name, acc.Type)
		qInfo, qErr := quota.FetchAccountQuota(acc, acc.ID == store.ActiveAccountID)
		if qErr != nil {
			fmt.Printf("  Failed to retrieve quota: %v\n", qErr)
			continue
		}
		_ = storage.AddAccount(acc)

		fmt.Printf("  Tier: %s\n", qInfo.Tier)
		if len(qInfo.Groups) > 0 {
			for _, grp := range qInfo.Groups {
				fmt.Printf("\n  %s:\n", grp.DisplayName)
				for _, b := range grp.Buckets {
					pct := int(b.RemainingFraction * 100)
					barLen := 20
					filledLen := int(float64(barLen) * b.RemainingFraction)
					if filledLen < 0 {
						filledLen = 0
					}
					if filledLen > barLen {
						filledLen = barLen
					}
					bar := strings.Repeat("█", filledLen) + strings.Repeat("░", barLen-filledLen)

					fmt.Printf("    %-26s [%s] %3d%%\n", b.DisplayName, bar, pct)
					if b.Description != "" {
						fmt.Printf("      %s\n", b.Description)
					}
				}
			}
		} else {
			for _, b := range qInfo.Buckets {
				pct := int(b.RemainingFraction * 100)
				barLen := 20
				filledLen := int(float64(barLen) * b.RemainingFraction)
				if filledLen < 0 {
					filledLen = 0
				}
				if filledLen > barLen {
					filledLen = barLen
				}
				bar := strings.Repeat("█", filledLen) + strings.Repeat("░", barLen-filledLen)

				label := b.DisplayName
				if label == "" {
					label = b.ModelID
				}
				fmt.Printf("  %-20s [%s] %3d%%\n", label, bar, pct)
				if b.Description != "" {
					fmt.Printf("    %s\n", b.Description)
				}
			}
		}
	}
	fmt.Println()
}

func handleLogin(storage *account.Storage, args []string) {
	headless := false
	for _, a := range args {
		if a == "--headless" || a == "--manual" || a == "-m" {
			headless = true
		}
	}

	ctx := context.Background()
	var acc *account.Account
	var err error

	if headless {
		acc, err = auth.LoginHeadless(ctx)
	} else {
		acc, err = auth.LoginWithBrowser(ctx)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		os.Exit(1)
	}

	// Fetch initial quota
	_, _ = quota.FetchAccountQuota(acc, true)

	if err := storage.AddAccount(acc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save account: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Successfully added Google account: %s (%s)\n", acc.Name, acc.Email)
	fmt.Printf("  Set as active account!\n\n")
}

func handleAddKey(storage *account.Storage, args []string) {
	fs := flag.NewFlagSet("add-key", flag.ExitOnError)
	nameFlag := fs.String("name", "", "Account name/label")
	keyFlag := fs.String("key", "", "Gemini API Key from Google AI Studio")
	rpmFlag := fs.Int("rpm", 15, "Requests Per Minute limit")
	rpdFlag := fs.Int("rpd", 1500, "Requests Per Day limit")
	skipVerify := fs.Bool("skip-verify", false, "Skip online key validation")
	_ = fs.Parse(args)

	name := *nameFlag
	key := *keyFlag

	if key == "" {
		fmt.Print("Enter Gemini API Key (from https://aistudio.google.com/apikey): ")
		_, _ = fmt.Scanln(&key)
		key = strings.TrimSpace(key)
	}

	if key == "" {
		fmt.Fprintln(os.Stderr, "Error: API key cannot be empty")
		os.Exit(1)
	}

	if name == "" {
		fmt.Print("Enter account label (e.g. 'Personal Studio Key'): ")
		_, _ = fmt.Scanln(&name)
		name = strings.TrimSpace(name)
		if name == "" {
			name = "Gemini Key (" + key[:min(8, len(key))] + "...)"
		}
	}

	if !*skipVerify {
		fmt.Print("Validating API key with Google...")
		if err := quota.ValidateAPIKey(key); err != nil {
			fmt.Fprintf(os.Stderr, "\nValidation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(" Valid!")
	}

	acc := &account.Account{
		ID:        fmt.Sprintf("key-%d", time.Now().Unix()),
		Type:      account.TypeAPIKey,
		Name:      name,
		APIKey:    key,
		RPMLimit:  *rpmFlag,
		RPDLimit:  *rpdFlag,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, _ = quota.FetchAccountQuota(acc, false)
	if err := storage.AddAccount(acc); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving account: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Added API key account '%s' successfully!\n", acc.Name)
}

func handleRemove(storage *account.Storage, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gemini-swap remove <account-id | email | name>")
		os.Exit(1)
	}
	id := args[0]
	if err := storage.RemoveAccount(id); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Removed account '%s'\n", id)
}

func handleExport(storage *account.Storage, args []string) {
	target := ""
	outPath := ""
	for i := 0; i < len(args); i++ {
		if (args[i] == "--file" || args[i] == "-o") && i+1 < len(args) {
			outPath = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") && target == "" {
			target = args[i]
		}
	}

	if target == "" {
		acc, err := storage.GetActiveAccount()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Usage: gemini-swap export [id|email] [--file <path>]")
			os.Exit(1)
		}
		target = acc.ID
	}

	code, path, err := storage.ExportAccount(target, outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Export failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Account exported successfully!")
	fmt.Printf("📁 Saved config file to: %s\n", path)
	fmt.Printf("📋 Share code (for sending to friend):\n\n%s\n\n", code)
	fmt.Println("Your friend can load it by running:")
	fmt.Printf("  gemini-swap import %s\n", path)
	fmt.Println("  OR")
	fmt.Println("  gemini-swap import <share-code>")
	fmt.Println()
}

func handleImport(storage *account.Storage, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gemini-swap import <share-code | path-to-config.json>")
		os.Exit(1)
	}

	source := args[0]
	acc, err := storage.ImportAccount(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Import failed: %v\n", err)
		os.Exit(1)
	}

	// Try fetching quota
	_, _ = quota.FetchAccountQuota(acc, false)
	_ = storage.AddAccount(acc)

	fmt.Printf("\n✓ Account '%s' (%s) imported successfully!\n", acc.Name, acc.Email)
	fmt.Println("To make it active, run:")
	fmt.Printf("  gemini-swap switch %s\n\n", acc.ID)
}

func handleExec(storage *account.Storage, args []string) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gemini-swap exec -- <command> [args...]")
		os.Exit(1)
	}

	acc, err := storage.GetActiveAccount()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting active account: %v\n", err)
		os.Exit(1)
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Inject active credentials
	env := os.Environ()
	if acc.Type == account.TypeAPIKey {
		env = append(env, "GEMINI_API_KEY="+acc.APIKey)
		env = append(env, "GOOGLE_API_KEY="+acc.APIKey)
	} else if acc.OAuth != nil {
		if acc.OAuth.IsExpired() {
			_ = auth.RefreshToken(acc.OAuth)
			_ = storage.AddAccount(acc)
		}
		env = append(env, "GOOGLE_GENAI_USE_GCA=true")
		env = append(env, "GOOGLE_CLOUD_ACCESS_TOKEN="+acc.OAuth.AccessToken)
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Execution error: %v\n", err)
		os.Exit(1)
	}
}

func handleProxy(storage *account.Storage, args []string) {
	fs := flag.NewFlagSet("proxy", flag.ExitOnError)
	portFlag := fs.Int("port", 8045, "Port to listen on")
	_ = fs.Parse(args)

	srv := proxy.NewServer(storage, *portFlag)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Starting Gemini Swap AI Proxy on port %d...\n", *portFlag)
	fmt.Println("Point Codex, Cline, Cursor, or curl to:")
	fmt.Printf("  http://127.0.0.1:%d/v1\n\n", *portFlag)

	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Proxy error: %v\n", err)
		os.Exit(1)
	}
}

func handleWeb(storage *account.Storage, args []string) {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	portFlag := fs.Int("port", 8080, "Port to listen on")
	_ = fs.Parse(args)

	srv := web.NewServer(storage, *portFlag)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Starting Gemini Swap Web Dashboard on http://0.0.0.0:%d...\n", *portFlag)
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
		os.Exit(1)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
