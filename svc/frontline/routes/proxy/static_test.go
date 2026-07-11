package handler_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	frontlinev1 "github.com/unkeyed/unkey/gen/proto/frontline/v1"
	"github.com/unkeyed/unkey/pkg/blobstore"
	"github.com/unkeyed/unkey/pkg/clock"
	"github.com/unkeyed/unkey/pkg/staticbundle"
	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/frontline/internal/errorpage"
	"github.com/unkeyed/unkey/svc/frontline/internal/policies"
	"github.com/unkeyed/unkey/svc/frontline/internal/router"
	"github.com/unkeyed/unkey/svc/frontline/internal/staticassets"
	"github.com/unkeyed/unkey/svc/frontline/middleware"
	handler "github.com/unkeyed/unkey/svc/frontline/routes/proxy"
)

func TestStaticArtifactDeliverySupportsPoliciesCachingAndSPA(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(root+"/assets", 0o755))
	require.NoError(t, os.WriteFile(root+"/index.html", []byte("<h1>LayerRail</h1>"), 0o644))
	require.NoError(t, os.WriteFile(root+"/assets/app.12345678.js", []byte("console.log('ready')"), 0o644))
	bundle, err := staticbundle.PackDirectory(root, staticbundle.DefaultLimits())
	require.NoError(t, err)

	store := blobstore.NewMemory()
	require.NoError(t, store.Put(context.Background(), "bundle.tar.gz", bundle.Bytes, blobstore.Metadata{}))
	resolver, err := staticassets.New(staticassets.Config{Store: store, MaxEntries: 2})
	require.NoError(t, err)

	evaluator := &recordingEvaluator{}
	decision := router.RouteDecision{
		Destination:   router.DestinationStaticArtifact,
		DeploymentID:  "dep_static",
		EnvironmentID: "env_static",
		WorkspaceID:   "ws_static",
		ProjectID:     "proj_static",
		AppID:         "app_static",
		Policies:      []*frontlinev1.Policy{{Enabled: true}},
		StaticArtifact: router.StaticArtifact{
			StorageKey:  "bundle.tar.gz",
			Digest:      bundle.Digest,
			SPAFallback: true,
		},
	}
	addr, stop := startStaticFrontline(t, decision, resolver, evaluator)
	t.Cleanup(stop)

	spa := staticRequest(t, addr, http.MethodGet, "/dashboard", "")
	defer func() { _ = spa.Body.Close() }()
	require.Equal(t, http.StatusOK, spa.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", spa.Header.Get("Content-Type"))
	require.Equal(t, "no-cache", spa.Header.Get("Cache-Control"))
	etag := spa.Header.Get("ETag")
	require.NotEmpty(t, etag)
	body, err := io.ReadAll(spa.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>LayerRail</h1>", string(body))

	notModified := staticRequest(t, addr, http.MethodGet, "/dashboard", etag)
	defer func() { _ = notModified.Body.Close() }()
	require.Equal(t, http.StatusNotModified, notModified.StatusCode)
	require.Zero(t, notModified.ContentLength)

	head := staticRequest(t, addr, http.MethodHead, "/assets/app.12345678.js", "")
	defer func() { _ = head.Body.Close() }()
	require.Equal(t, http.StatusOK, head.StatusCode)
	require.Equal(t, "public, max-age=31536000, immutable", head.Header.Get("Cache-Control"))
	headBody, err := io.ReadAll(head.Body)
	require.NoError(t, err)
	require.Empty(t, headBody)

	missing := staticRequest(t, addr, http.MethodGet, "/missing.txt", "")
	defer func() { _ = missing.Body.Close() }()
	require.Equal(t, http.StatusNotFound, missing.StatusCode)

	method := staticRequest(t, addr, http.MethodPost, "/", "")
	defer func() { _ = method.Body.Close() }()
	require.Equal(t, http.StatusMethodNotAllowed, method.StatusCode)
	require.Equal(t, "GET, HEAD", method.Header.Get("Allow"))
	require.Equal(t, int64(5), evaluator.calls.Load(), "policies must run before every direct static response")
}

type recordingEvaluator struct {
	calls atomic.Int64
}

func (e *recordingEvaluator) Evaluate(
	_ context.Context,
	_ *zen.Session,
	_ *http.Request,
	_ string,
	_ []*frontlinev1.Policy,
) (policies.Result, error) {
	e.calls.Add(1)
	return policies.Result{}, nil
}

func startStaticFrontline(
	t *testing.T,
	decision router.RouteDecision,
	resolver staticassets.Resolver,
	evaluator policies.Evaluator,
) (string, func()) {
	t.Helper()

	h := &handler.Handler{
		RouterService: &stubRouter{decision: decision},
		Engine:        evaluator,
		Clock:         clock.New(),
		StaticAssets:  resolver,
	}
	zenSrv, err := zen.New(zen.Config{
		ReadTimeout:        -1,
		WriteTimeout:       -1,
		MaxRequestBodySize: 0,
	})
	require.NoError(t, err)
	zenSrv.RegisterRoute([]zen.Middleware{
		zen.WithPanicRecovery(),
		middleware.WithReservedHeaderStrip(),
		zen.WithLogging(),
		middleware.WithObservability(errorpage.NewRenderer()),
	}, h)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = zenSrv.Serve(ctx, listener) }()
	waitForListener(t, listener.Addr().String())

	return listener.Addr().String(), func() {
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = zenSrv.Shutdown(shutdownCtx)
	}
}

func staticRequest(t *testing.T, address string, method string, path string, etag string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+address+path, nil)
	require.NoError(t, err)
	req.Host = "static-test.example.com"
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return response
}
