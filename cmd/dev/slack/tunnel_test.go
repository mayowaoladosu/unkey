package slack

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNgrokURLFromLine(t *testing.T) {
	line := `t=2024-06-01T00:00:00+0000 lvl=info msg="started tunnel" obj=tunnels name=command_line addr=http://localhost:3000 url=https://abcd-1-2-3-4.ngrok-free.app`
	require.Equal(t, "https://abcd-1-2-3-4.ngrok-free.app", ngrokURLFromLine(line))

	// Non-tunnel lines yield nothing, incl. the local addr (http, not https).
	require.Equal(t, "", ngrokURLFromLine(`t=... lvl=info msg="starting web service" obj=web addr=127.0.0.1:4040`))
	require.Equal(t, "", ngrokURLFromLine(`t=... lvl=info msg="tunnel session started"`))
}

func TestIsNgrokErrorLine(t *testing.T) {
	require.True(t, isNgrokErrorLine(`t=... lvl=eror msg="failed to reconnect session"`))
	require.True(t, isNgrokErrorLine(`t=... lvl=crit msg="something fatal"`))
	require.False(t, isNgrokErrorLine(`t=... lvl=info msg="started tunnel" url=https://x.ngrok-free.app`))
	require.False(t, isNgrokErrorLine(`t=... lvl=warn msg="deprecation"`))
}

func TestWaitForNgrokURL_HappyPath(t *testing.T) {
	out := strings.Join([]string{
		`t=... lvl=info msg="no configuration paths supplied"`,
		`t=... lvl=info msg="starting web service" obj=web addr=127.0.0.1:4040`,
		`t=... lvl=info msg="started tunnel" obj=tunnels name=command_line addr=http://localhost:3000 url=https://abcd.ngrok-free.app`,
	}, "\n")

	url, err := waitForNgrokURL(strings.NewReader(out))
	require.NoError(t, err)
	require.Equal(t, "https://abcd.ngrok-free.app", url)
}

func TestWaitForNgrokURL_SurfacesSessionLimit(t *testing.T) {
	// The exact failure the user hit while github-tunnel held the one free
	// agent session — now surfaced instead of a blind timeout.
	out := `t=... lvl=eror msg="failed to reconnect session" obj=csess err="authentication failed: Your account is limited to 1 simultaneous ngrok agent sessions."`

	_, err := waitForNgrokURL(strings.NewReader(out))
	require.Error(t, err)
	require.Contains(t, err.Error(), "simultaneous ngrok agent sessions")
}

func TestWaitForNgrokURL_NoURLBeforeExit(t *testing.T) {
	_, err := waitForNgrokURL(strings.NewReader(`t=... lvl=info msg="just some noise"`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "exited before reporting")
}
