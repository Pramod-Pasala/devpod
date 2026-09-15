package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/loft-sh/devpod/pkg/agent/tunnelserver"
	"github.com/loft-sh/log"
)

// Harness acting as the DevPod *client* side: spawns the credentials-server
// in the pod via kubectl and serves its tunnel (Log, GitUser, ...),
// printing every pod-side log line so we can see where it dies.

func main() {
	cmd := exec.Command("kubectl", "exec", "-i", "-n", "pasala-ns",
		"devpod-default-ml-a6f2f", "-c", "devpod", "--",
		"/tmp/devpod", "agent", "container", "credentials-server",
		"--user", "vscode", "--configure-git-helper", "--configure-docker-helper", "--debug")
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}

	logger := log.Default
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	srvErr := make(chan error, 1)
	go func() {
		// allowGitCredentials=true, allowDockerCredentials=true: reply to
		// GitUser from this machine's git config (may be empty -> pod then
		// keeps its own), and let credential lookups hit the Unimplemented
		// fallback if any.
		srvErr <- tunnelserver.RunServicesServer(ctx, stdout, stdin, true, true, nil, nil, logger)
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-srvErr:
		fmt.Fprintf(os.Stderr, "[HARNESS] tunnel server ended: %v\n", err)
	case err := <-done:
		fmt.Fprintf(os.Stderr, "[HARNESS] pod-side credentials-server exited: %v\n", err)
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "[HARNESS] 60s timeout reached")
	}

	// after exit, inspect what got written inside the pod
	_ = cmd.Process.Kill()
	inspect, _ := exec.Command("kubectl", "exec", "-n", "pasala-ns",
		"devpod-default-ml-a6f2f", "-c", "devpod", "--", "sh", "-c",
		`echo "=== .gitconfig ==="; cat /home/vscode/.gitconfig`).CombinedOutput()
	fmt.Fprintf(os.Stderr, "%s\n", inspect)
}
