// Package tunnel implements `dev tunnel`, which runs the GitHub webhook tunnel
// and the Slack (dashboard) tunnel in a SINGLE ngrok agent.
//
// ngrok's free plan allows only one simultaneous agent session, so two separate
// `ngrok http` processes (as `dev github tunnel` and `dev slack tunnel` start)
// cannot both run. One agent can, however, serve multiple named tunnels via
// `ngrok start --all` with a config. This command generates a tunnels config,
// merges it with the user's default config (for the authtoken) using ngrok's
// multi-`--config` merge, reads each tunnel's URL by name from ngrok's logfmt
// output, PATCHes the GitHub App webhook, and prints the Slack URLs.
package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/unkeyed/unkey/cmd/dev/github"
	"github.com/unkeyed/unkey/pkg/cli"
)

// Cmd is the `dev tunnel` command.
var Cmd = &cli.Command{
	Name:  "tunnel",
	Usage: "Run the GitHub webhook and Slack (dashboard) tunnels together in one ngrok agent",
	Flags: []cli.Flag{
		cli.String("dashboard-port", "Local dashboard port (Slack redirect + interactivity)", cli.Default("3000")),
		cli.String("github-port", "Local ctrl-api port (GitHub webhook)", cli.Default("7091")),
		cli.String("env-file", "Path to .env.github", cli.Default("dev/.env.github")),
		cli.String("pem-file", "Path to .github-private-key.pem", cli.Default("dev/.github-private-key.pem")),
	},
	Action: run,
}

func run(_ context.Context, cmd *cli.Command) error {
	dashPort := cmd.String("dashboard-port")
	ghPort := cmd.String("github-port")
	envFile := cmd.String("env-file")
	pemFile := cmd.String("pem-file")

	if _, err := exec.LookPath("ngrok"); err != nil {
		panic("ngrok is not installed or not in PATH\n\nInstall it from: https://ngrok.com/download")
	}

	// GitHub tunneling needs the app id + private key written by `dev github
	// setup`; skip it (Slack-only) when they're absent rather than failing.
	githubEnabled := fileExists(envFile) && fileExists(pemFile)

	defaultCfg, err := defaultNgrokConfigPath()
	if err != nil {
		return err
	}

	tmpCfg, err := writeTunnelsConfig(dashPort, ghPort, githubEnabled)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpCfg) }()

	expected := []string{"slack"}
	if githubEnabled {
		fmt.Printf("Starting one ngrok agent: dashboard:%s (slack) + ctrl:%s (github)...\n", dashPort, ghPort)
		expected = append(expected, "github")
	} else {
		fmt.Printf("Starting one ngrok agent: dashboard:%s (slack). GitHub tunnel skipped — %s / %s not found (run `go run . dev github setup`).\n", dashPort, envFile, pemFile)
	}

	// One agent, all tunnels from the merged config. The default config supplies
	// the authtoken; tmpCfg supplies the tunnel definitions.
	ngrok := exec.Command("ngrok", "start", "--all",
		"--log", "stdout", "--log-format", "logfmt",
		"--config", defaultCfg, "--config", tmpCfg)
	stdout, err := ngrok.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to capture ngrok output: %w", err)
	}
	if err := ngrok.Start(); err != nil {
		return fmt.Errorf("failed to start ngrok: %w", err)
	}

	urls, err := waitForTunnels(stdout, expected)
	if err != nil {
		_ = ngrok.Process.Kill()
		return err
	}

	fmt.Printf("\n✔ Tunnels running (press Ctrl+C to stop)\n")

	if githubEnabled {
		webhookURL := urls["github"] + "/webhooks/github"
		fmt.Printf("\nGitHub webhook: %s\n", webhookURL)
		if updErr := github.UpdateWebhookURL(envFile, pemFile, webhookURL); updErr != nil {
			fmt.Printf("  ⚠ could not update GitHub App webhook: %v\n", updErr)
		} else {
			fmt.Printf("  ✔ GitHub App webhook updated\n")
		}
	}

	slackURL := urls["slack"]
	redirectURL := slackURL + "/integrations/slack/callback"
	interactivityURL := slackURL + "/api/webhooks/slack"
	fmt.Printf("\nSlack tunnel: %s\n", slackURL)
	fmt.Printf("Configure in your Slack app (https://api.slack.com/apps):\n")
	fmt.Printf("  OAuth & Permissions → Redirect URL:   %s\n", redirectURL)
	fmt.Printf("  Interactivity → Request URL:          %s\n", interactivityURL)
	fmt.Printf("Set in the dashboard environment:\n")
	fmt.Printf("  SLACK_REDIRECT_URI=%s\n\n", redirectURL)

	// Keep running until killed
	_ = ngrok.Wait()
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// defaultNgrokConfigPath asks ngrok where its config lives so we can merge our
// tunnels config with it (and inherit the authtoken).
func defaultNgrokConfigPath() (string, error) {
	out, err := exec.Command("ngrok", "config", "check").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ngrok config check failed: %s\n\nAuthenticate first: ngrok config add-authtoken <token>", strings.TrimSpace(string(out)))
	}
	const marker = "configuration file at "
	for _, line := range strings.Split(string(out), "\n") {
		if i := strings.Index(line, marker); i >= 0 {
			return strings.TrimSpace(line[i+len(marker):]), nil
		}
	}
	return "", fmt.Errorf("could not determine ngrok config path from: %s", strings.TrimSpace(string(out)))
}

// writeTunnelsConfig writes a temp ngrok v3 config defining the enabled tunnels.
func writeTunnelsConfig(dashPort, ghPort string, githubEnabled bool) (string, error) {
	var b strings.Builder
	b.WriteString("version: \"3\"\n")
	b.WriteString("tunnels:\n")
	b.WriteString("  slack:\n    proto: http\n    addr: " + dashPort + "\n")
	if githubEnabled {
		b.WriteString("  github:\n    proto: http\n    addr: " + ghPort + "\n")
	}

	f, err := os.CreateTemp("", "unkey-ngrok-*.yml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp ngrok config: %w", err)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("failed to write ngrok config: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to close ngrok config: %w", err)
	}
	return f.Name(), nil
}

// waitForTunnels reads ngrok's logfmt output until every expected tunnel has
// reported a URL, or an error/timeout occurs. It keeps draining afterwards so
// ngrok never blocks writing to a full pipe.
func waitForTunnels(stdout io.Reader, expected []string) (map[string]string, error) {
	type result struct {
		urls map[string]string
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		found := map[string]string{}
		scanner := bufio.NewScanner(stdout)
		reported := false
		for scanner.Scan() {
			line := scanner.Text()
			if reported {
				continue // drain
			}
			if isNgrokErrorLine(line) {
				ch <- result{urls: nil, err: fmt.Errorf("ngrok: %s", ngrokErrorFromLine(line))}
				reported = true
				continue
			}
			if name, url := tunnelFromLine(line); name != "" && url != "" {
				found[name] = url
				if hasAll(found, expected) {
					ch <- result{urls: copyMap(found), err: nil}
					reported = true
				}
			}
		}
		if !reported {
			ch <- result{urls: nil, err: fmt.Errorf("ngrok exited before reporting tunnels %v", expected)}
		}
	}()

	select {
	case r := <-ch:
		return r.urls, r.err
	case <-time.After(25 * time.Second):
		return nil, fmt.Errorf("timed out waiting for ngrok tunnels\n\nMake sure ngrok is authenticated: ngrok config add-authtoken <token>")
	}
}

// tunnelFromLine extracts (name, url) from a logfmt `started tunnel` line.
func tunnelFromLine(line string) (string, string) {
	var name, url string
	for _, field := range strings.Fields(line) {
		if v, ok := strings.CutPrefix(field, "name="); ok {
			name = strings.Trim(v, `"`)
		}
		if v, ok := strings.CutPrefix(field, "url="); ok {
			v = strings.Trim(v, `"`)
			if strings.HasPrefix(v, "https://") {
				url = v
			}
		}
	}
	return name, url
}

func isNgrokErrorLine(line string) bool {
	return strings.Contains(line, "lvl=eror") || strings.Contains(line, "lvl=crit")
}

func ngrokErrorFromLine(line string) string {
	for _, key := range []string{"err=", "msg="} {
		if i := strings.Index(line, key); i >= 0 {
			return strings.TrimSpace(strings.Trim(line[i+len(key):], `"`))
		}
	}
	return strings.TrimSpace(line)
}

func hasAll(found map[string]string, expected []string) bool {
	for _, name := range expected {
		if found[name] == "" {
			return false
		}
	}
	return true
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
