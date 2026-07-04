package tunnel

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTunnelFromLine(t *testing.T) {
	name, url := tunnelFromLine(`t=... lvl=info msg="started tunnel" obj=tunnels name=slack addr=http://localhost:3000 url=https://slack-abc.ngrok-free.app`)
	require.Equal(t, "slack", name)
	require.Equal(t, "https://slack-abc.ngrok-free.app", url)

	name, url = tunnelFromLine(`t=... lvl=info msg="started tunnel" obj=tunnels name=github addr=http://localhost:7091 url=https://gh-xyz.ngrok-free.app`)
	require.Equal(t, "github", name)
	require.Equal(t, "https://gh-xyz.ngrok-free.app", url)

	// A web-service line has an http addr but no https url → no tunnel.
	name, url = tunnelFromLine(`t=... lvl=info msg="starting web service" obj=web addr=127.0.0.1:4040`)
	require.Empty(t, name+url)
}

func TestWaitForTunnels_BothReported(t *testing.T) {
	out := strings.Join([]string{
		`t=... lvl=info msg="starting web service" obj=web addr=127.0.0.1:4040`,
		`t=... lvl=info msg="started tunnel" obj=tunnels name=slack addr=http://localhost:3000 url=https://slack-abc.ngrok-free.app`,
		`t=... lvl=info msg="started tunnel" obj=tunnels name=github addr=http://localhost:7091 url=https://gh-xyz.ngrok-free.app`,
	}, "\n")

	urls, err := waitForTunnels(strings.NewReader(out), []string{"slack", "github"})
	require.NoError(t, err)
	require.Equal(t, "https://slack-abc.ngrok-free.app", urls["slack"])
	require.Equal(t, "https://gh-xyz.ngrok-free.app", urls["github"])
}

func TestWaitForTunnels_SlackOnly(t *testing.T) {
	out := `t=... lvl=info msg="started tunnel" obj=tunnels name=slack addr=http://localhost:3000 url=https://slack-abc.ngrok-free.app`
	urls, err := waitForTunnels(strings.NewReader(out), []string{"slack"})
	require.NoError(t, err)
	require.Equal(t, "https://slack-abc.ngrok-free.app", urls["slack"])
}

func TestWaitForTunnels_SurfacesSessionLimit(t *testing.T) {
	out := `t=... lvl=eror msg="failed to reconnect session" obj=csess err="authentication failed: Your account is limited to 1 simultaneous ngrok agent sessions."`
	_, err := waitForTunnels(strings.NewReader(out), []string{"slack", "github"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "simultaneous ngrok agent sessions")
}

func TestWaitForTunnels_IncompleteBeforeExit(t *testing.T) {
	// Only slack reported, but github also expected.
	out := `t=... lvl=info msg="started tunnel" obj=tunnels name=slack addr=http://localhost:3000 url=https://slack-abc.ngrok-free.app`
	_, err := waitForTunnels(strings.NewReader(out), []string{"slack", "github"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exited before reporting")
}

func TestWriteTunnelsConfig(t *testing.T) {
	withGH, err := writeTunnelsConfig("3000", "7091", true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(withGH) })
	data, err := os.ReadFile(withGH)
	require.NoError(t, err)
	body := string(data)
	require.Contains(t, body, `version: "3"`)
	require.Contains(t, body, "slack:")
	require.Contains(t, body, "addr: 3000")
	require.Contains(t, body, "github:")
	require.Contains(t, body, "addr: 7091")

	slackOnly, err := writeTunnelsConfig("3000", "7091", false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(slackOnly) })
	data, err = os.ReadFile(slackOnly)
	require.NoError(t, err)
	require.Contains(t, string(data), "slack:")
	require.NotContains(t, string(data), "github:")
}
