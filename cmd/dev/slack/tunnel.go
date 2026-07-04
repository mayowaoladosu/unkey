package slack

// tunnel.go implements the `dev slack tunnel` subcommand.
//
// Slack's OAuth redirect and interactivity webhook must be reachable over a
// public HTTPS URL, which localhost is not. This starts an ngrok tunnel to the
// dashboard (where both the OAuth callback page and the /api/webhooks/slack
// interactivity route live) and prints the two URLs to configure in the Slack
// app plus the SLACK_REDIRECT_URI env value.
//
// Unlike `dev github tunnel`, there is no provider API to auto-register the URL:
// Slack app URLs are set in the app console / manifest, so the command prints
// them for the developer to paste.
//
// We read the assigned URL (and any startup error) directly from ngrok's
// structured log output rather than polling its web-inspection API on a fixed
// port. That fixed port (4040) shifts when a second agent runs, and — more to
// the point — ngrok's free plan rejects a second simultaneous agent session, so
// running this alongside `dev github tunnel` surfaces a clear ngrok error here
// instead of a mysterious timeout.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/unkeyed/unkey/pkg/cli"
)

// Cmd is the `dev slack` command group.
var Cmd = &cli.Command{
	Name:  "slack",
	Usage: "Slack local development tools",
	Commands: []*cli.Command{
		tunnelCmd,
	},
}

var tunnelCmd = &cli.Command{
	Name:  "tunnel",
	Usage: "Start an ngrok tunnel to the dashboard for Slack OAuth redirect + interactivity webhook testing",
	Flags: []cli.Flag{
		cli.String("port", "Local dashboard port to tunnel", cli.Default("3000")),
	},
	Action: startTunnel,
}

func startTunnel(_ context.Context, cmd *cli.Command) error {
	port := cmd.String("port")

	// Fail fast if ngrok is not installed
	if _, err := exec.LookPath("ngrok"); err != nil {
		panic("ngrok is not installed or not in PATH\n\nInstall it from: https://ngrok.com/download")
	}

	// logfmt on stdout makes the assigned URL and any error machine-readable.
	fmt.Printf("Starting ngrok tunnel to localhost:%s (dashboard)...\n", port)
	ngrok := exec.Command("ngrok", "http", port, "--log", "stdout", "--log-format", "logfmt")
	stdout, err := ngrok.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to capture ngrok output: %w", err)
	}
	if err := ngrok.Start(); err != nil {
		return fmt.Errorf("failed to start ngrok: %w", err)
	}

	publicURL, err := waitForNgrokURL(stdout)
	if err != nil {
		_ = ngrok.Process.Kill()
		return err
	}

	redirectURL := publicURL + "/integrations/slack/callback"
	interactivityURL := publicURL + "/api/webhooks/slack"

	fmt.Printf("\n✔ Tunnel running (press Ctrl+C to stop)\n\n")
	fmt.Printf("Tunnel URL: %s\n\n", publicURL)
	fmt.Printf("Configure these in your Slack app (https://api.slack.com/apps):\n")
	fmt.Printf("  OAuth & Permissions → Redirect URL:   %s\n", redirectURL)
	fmt.Printf("  Interactivity → Request URL:          %s\n\n", interactivityURL)
	fmt.Printf("Then set in the dashboard environment:\n")
	fmt.Printf("  SLACK_REDIRECT_URI=%s\n", redirectURL)

	// Keep running until killed
	_ = ngrok.Wait()
	return nil
}

// waitForNgrokURL reads ngrok's logfmt output until it reports a tunnel URL or
// an error, whichever comes first. It keeps draining the reader afterwards so
// ngrok never blocks writing further logs to a full pipe.
func waitForNgrokURL(stdout io.Reader) (string, error) {
	type result struct {
		url string
		err error
	}
	ch := make(chan result, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		reported := false
		for scanner.Scan() {
			line := scanner.Text()
			if reported {
				continue // drain remaining output so ngrok doesn't block
			}
			if u := ngrokURLFromLine(line); u != "" {
				ch <- result{url: u, err: nil}
				reported = true
				continue
			}
			if isNgrokErrorLine(line) {
				ch <- result{url: "", err: fmt.Errorf("ngrok: %s", ngrokErrorFromLine(line))}
				reported = true
				continue
			}
		}
		if !reported {
			ch <- result{url: "", err: fmt.Errorf("ngrok exited before reporting a tunnel URL")}
		}
	}()

	select {
	case r := <-ch:
		return r.url, r.err
	case <-time.After(20 * time.Second):
		return "", fmt.Errorf("timed out waiting for ngrok tunnel\n\nMake sure ngrok is authenticated (ngrok config add-authtoken <token>). Note: ngrok's free plan allows only one agent at a time, so stop the github-tunnel before running this")
	}
}

// ngrokURLFromLine extracts the public https URL from a logfmt line such as:
// `t=... lvl=info msg="started tunnel" ... url=https://abcd.ngrok-free.app`.
func ngrokURLFromLine(line string) string {
	for _, field := range strings.Fields(line) {
		if v, ok := strings.CutPrefix(field, "url="); ok {
			v = strings.Trim(v, `"`)
			if strings.HasPrefix(v, "https://") {
				return v
			}
		}
	}
	return ""
}

// isNgrokErrorLine reports whether a logfmt line is an error or critical entry.
func isNgrokErrorLine(line string) bool {
	return strings.Contains(line, "lvl=eror") || strings.Contains(line, "lvl=crit")
}

// ngrokErrorFromLine pulls the human-readable message out of an error log line,
// preferring the err= field and falling back to the whole line.
func ngrokErrorFromLine(line string) string {
	for _, key := range []string{"err=", `msg=`} {
		if i := strings.Index(line, key); i >= 0 {
			return strings.TrimSpace(strings.Trim(line[i+len(key):], `"`))
		}
	}
	return strings.TrimSpace(line)
}
