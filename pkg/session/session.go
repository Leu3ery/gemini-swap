// Package session orchestrates safe account switching across Gemini CLI and
// the Antigravity IDE: token refresh, session backup, file sync, IDE reload,
// and post-switch verification.
package session

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gemini-swap/pkg/account"
	"gemini-swap/pkg/auth"
)

// Options controls SwitchTo behavior.
type Options struct {
	// NoRestart writes credential files but does not signal the running
	// Antigravity IDE (it will pick them up on next window reload).
	NoRestart bool
	// SkipVerify skips the post-restart GetUserStatus identity check.
	SkipVerify bool
}

// SwitchTo activates the given account (by ID, email or name) in the store,
// ~/.gemini/*, and — for OAuth accounts — the Antigravity session.
//
// Safety measures (learned the hard way):
//   - expired access tokens are refreshed BEFORE writing, using the correct
//     OAuth client, so the IDE receives maximum-lived credentials;
//   - the previous IDE session is backed up and can be restored with
//     `gemini-swap restore-antigravity` if the IDE rejects the new token;
//   - active_account_id is committed only after the respawned language server
//     reports the target email from GetUserStatus.
func SwitchTo(storage *account.Storage, idOrEmail string, opts Options) (*account.Account, error) {
	store, err := storage.Load()
	if err != nil {
		return nil, err
	}

	var target *account.Account
	for _, acc := range store.Accounts {
		if acc.ID == idOrEmail || acc.Email == idOrEmail || acc.Name == idOrEmail {
			target = acc
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("account not found: %s", idOrEmail)
	}

	isOAuth := target.Type == account.TypeOAuth && target.OAuth != nil
	// IDE files accept only tokens the language_server can refresh. Only
	// tokens explicitly minted by Antigravity are safe. Older accounts with
	// no provenance were normally imported from Gemini CLI and must be
	// re-authorized instead of being written optimistically.
	touchesIDE := isOAuth && target.OAuth.Client == "antigravity"
	if isOAuth && !touchesIDE {
		return nil, fmt.Errorf("account %s is not authorized for Antigravity; sign in for Antigravity once, then switch again", target.Email)
	}

	// 1. Refresh an expired access token first, so every consumer gets a
	//    fresh token. auth.RefreshToken tries the Gemini client first, then
	//    the Antigravity one — whichever minted the refresh token wins.
	if isOAuth && target.OAuth.IsExpired() && target.OAuth.RefreshToken != "" {
		if err := auth.RefreshToken(target.OAuth); err == nil {
			target.UpdatedAt = time.Now()
			_ = storage.AddAccount(target)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: could not refresh expired token for %s: %v. The IDE session may end soon.\n", target.Email, err)
		}
	}

	// 2. Back up the live IDE session before overwriting it.
	backupPath := ""
	if touchesIDE {
		if p, err := account.BackupAntigravitySession(); err == nil {
			backupPath = p
		}
	}

	if !touchesIDE {
		return storage.SetActiveAccount(target.ID)
	}

	// 3. Stage credentials first, but deliberately do not update
	// active_account_id yet. The macOS app watches that file and would otherwise
	// show Active before Antigravity has actually finished switching.
	if err := account.SyncToGeminiCLI(target); err != nil {
		return nil, fmt.Errorf("failed to stage Gemini credentials: %w", err)
	}
	if err := account.SyncToAntigravity(target); err != nil {
		return nil, fmt.Errorf("failed to stage Antigravity credentials: %w", err)
	}

	// 4. Ask the running IDE to reload credentials.
	restartEnvOff := os.Getenv("GEMINI_SWAP_NO_RESTART") == "1"
	if !opts.NoRestart && !restartEnvOff {
		if err := account.RestartAntigravityLanguageServer(); err != nil {
			return nil, fmt.Errorf("credentials were staged, but Antigravity could not reload: %w", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Skipping Antigravity reload (--no-restart). Reload the Antigravity window to apply.\n")
		return storage.CommitActiveAccount(target.ID)
	}

	// 5. Wait for the respawned IDE server to report the target identity. Only
	// then commit active_account_id, which is the UI's success signal.
	if !opts.SkipVerify {
		if err := verifySwitch(target, backupPath); err != nil {
			return nil, err
		}
	}

	return storage.CommitActiveAccount(target.ID)
}

// verifySwitch waits until Antigravity itself reports the target account.
func verifySwitch(active *account.Account, backupPath string) error {
	restoreHint := "If Antigravity signed you out, sign in inside the IDE, then run `gemini-swap resync`."
	if backupPath != "" {
		restoreHint = fmt.Sprintf("Previous session backup: %s (restore with `gemini-swap restore-antigravity`).", backupPath)
	}
	deadline := time.Now().Add(20 * time.Second)
	lastEmail := ""
	var lastErr error
	for {
		email, err := account.GetAntigravityUserEmail()
		if err == nil {
			lastEmail = email
			if strings.EqualFold(email, active.Email) {
				return nil
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastEmail != "" {
				return fmt.Errorf("Antigravity still reports %s instead of %s after 20s. %s", lastEmail, active.Email, restoreHint)
			}
			return fmt.Errorf("Antigravity did not become ready after 20s (%v). %s", lastErr, restoreHint)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
