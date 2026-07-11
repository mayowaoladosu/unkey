package deploy

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFunctionRuntimeCommand(t *testing.T) {
	node, err := functionRuntimeCommand("nodejs22")
	require.NoError(t, err)
	require.Equal(t, []string{"node", "-e"}, node[:2])
	require.Contains(t, node[2], "LAYER_RAIL_HANDLER")
	require.Contains(t, node[2], "createServer")

	python, err := functionRuntimeCommand("python3.12")
	require.NoError(t, err)
	require.Equal(t, []string{"python3", "-c"}, python[:2])
	require.Contains(t, python[2], "ThreadingHTTPServer")

	_, err = functionRuntimeCommand("ruby3.3")
	require.ErrorContains(t, err, "unsupported function runtime")
}

func TestFunctionRuntimeAdaptersServeHandlers(t *testing.T) {
	tests := []struct {
		name     string
		runtime  string
		filename string
		source   string
	}{
		{name: "node", runtime: "nodejs22", filename: "handler.js", source: `exports.handler=async(event)=>({statusCode:201,headers:{"content-type":"text/plain"},body:"node:"+event.method})`},
		{name: "python", runtime: "python3.12", filename: "handler.py", source: `def handler(event, context): return {"statusCode": 201, "headers": {"content-type": "text/plain"}, "body": "python:" + event["method"]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command, err := functionRuntimeCommand(test.runtime)
			require.NoError(t, err)
			if _, err := exec.LookPath(command[0]); err != nil {
				t.Skipf("%s runtime is not installed", command[0])
			}
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, test.filename), []byte(test.source), 0o600))
			port := availablePort(t)
			process := exec.Command(command[0], command[1:]...) //nolint:gosec // fixed runtime adapter command
			process.Dir = dir
			process.Env = append(os.Environ(), "PORT="+strconv.Itoa(port), "LAYER_RAIL_HANDLER=handler.handler")
			var stderr bytes.Buffer
			process.Stderr = &stderr
			require.NoError(t, process.Start())
			t.Cleanup(func() {
				if process.Process != nil {
					_ = process.Process.Kill()
					_, _ = process.Process.Wait()
				}
			})

			var response *http.Response
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				response, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/hello", port)) //nolint:gosec,noctx // loopback test server
				if err == nil {
					break
				}
				time.Sleep(25 * time.Millisecond)
			}
			require.NoError(t, err, stderr.String())
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, response.StatusCode, stderr.String())
			require.Equal(t, test.name+":GET", string(body))
		})
	}
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
