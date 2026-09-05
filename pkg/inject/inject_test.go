package inject

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/loft-sh/log"
)

// TestInjectHandshakeRoundTrip verifies the client side of the inject
// handshake: the remote script prints "ping", the client must receive it
// promptly and its "pong" reply must be delivered, so that "done" follows.
// This is the path that hangs on Windows ProxyCommand connections
// (loft-sh/devpod#972): any buffering or starvation here makes the
// agent injection loop until the timeout with no visible error.
func TestInjectHandshakeRoundTrip(t *testing.T) {
	execFn := func(ctx context.Context, command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
		// run the script through a real local shell, like the SSH provider
		// connection does on the remote host. `command` is the fully rendered
		// inject script with Params.Command appended at the end
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	}

	wasExecuted, err := InjectAndExecute(
		context.Background(),
		execFn,
		func(arm bool) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		},
		&Params{
			AgentRemotePath: "/tmp/devpod-test-inject/agent",
			DownloadURLs:    &DownloadURLs{Amd: "unused", Arm: "unused", Base: "unused"},
			ExistsCheck:     "true", // agent "already exists": skip install branch
			// the rendered script becomes:
			//   echo ping; read -r PING; ... (handshake)
			//   ... install skipped via ExistsCheck ...
			//   echo done
			//   {{ .Command }}   <-- our user command runs here
			Command: `echo ping-verified-via-command`,
		},
		strings.NewReader(""), // client stdin after handshake
		io.Discard,            // client stdout
		io.Discard,            // client stderr
		time.Second*10,        // handshake timeout
		log.Discard,
	)
	if err != nil {
		t.Fatalf("InjectAndExecute failed: %v", err)
	}
	if !wasExecuted {
		t.Fatal("expected wasExecuted=true: the handshake completed and the script ran to completion")
	}
}
