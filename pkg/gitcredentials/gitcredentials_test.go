package gitcredentials

import (
	"strings"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

// TestSetUserWritesUsersOwnGitconfig verifies that SetUser writes to the
// target user's own ~/.gitconfig (via `git config --file`) instead of
// `su <user> -c "git config --global"`, which inherits the current
// environment (HOME=/root) and fails with
// "fatal: error reading '/root/.git'" for non-root users.
// See loft-sh/devpod#972.
func TestSetUserWritesUsersOwnGitconfig(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root to switch to another user")
	}

	// pick a real non-root user on the system (usually "daemon" or "nobody")
	target := ""
	for _, candidate := range []string{"daemon", "nobody", "ubuntu", "node"} {
		if _, err := user.Lookup(candidate); err == nil {
			target = candidate
			break
		}
	}
	if target == "" {
		t.Skip("no non-root user found to test with")
	}

	u, err := user.Lookup(target)
	if err != nil {
		t.Fatal(err)
	}
	gitconfig := filepath.Join(u.HomeDir, ".gitconfig")

	// remember original state so we can restore
	orig, readErr := os.ReadFile(gitconfig)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("read %s: %v", gitconfig, readErr)
	}

	// also capture the environment we're running in: HOME must NOT
	// point at the target user's home, otherwise the test can't catch
	// the old su-based bug (the old code passed our HOME through)
	if home, _ := os.UserHomeDir(); home == u.HomeDir {
		t.Skip("test needs HOME to differ from target user's home")
	}

	// run SetUser as root with our (root) HOME
	err = SetUser(target, &GitUser{
		Name:  "DevPod Test",
		Email: "devpod-test@example.com",
	})
	if err != nil {
		t.Fatalf("SetUser failed: %v", err)
	}

	// verify the value actually landed in the target user's gitconfig
	content, err := os.ReadFile(gitconfig)
	if err != nil {
		t.Fatalf("target gitconfig not written: %v", err)
	}
	if !strings.Contains(string(content), "devpod-test@example.com") {
		t.Fatalf("expected email in %s, got:\n%s", gitconfig, content)
	}

	// restore original state
	if os.IsNotExist(readErr) {
		_ = os.Remove(gitconfig)
	} else {
		_ = os.WriteFile(gitconfig, orig, 0644)
	}
}
