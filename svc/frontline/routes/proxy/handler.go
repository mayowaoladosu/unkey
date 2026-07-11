package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/unkeyed/unkey/pkg/clock"
	"github.com/unkeyed/unkey/pkg/codes"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/frontline/internal/policies"
	"github.com/unkeyed/unkey/svc/frontline/internal/proxy"
	"github.com/unkeyed/unkey/svc/frontline/internal/router"
	"github.com/unkeyed/unkey/svc/frontline/internal/staticassets"
)

type Handler struct {
	RouterService router.Service
	ProxyService  proxy.Service
	Engine        policies.Evaluator
	Clock         clock.Clock
	StaticAssets  staticassets.Resolver
}

func (h *Handler) Method() string {
	return zen.CATCHALL
}

func (h *Handler) Path() string {
	return "/{path...}"
}

func (h *Handler) Handle(ctx context.Context, sess *zen.Session) error {
	startTime := h.Clock.Now()
	ctx = proxy.WithRequestStartTime(ctx, startTime)

	hostname := proxy.ExtractHostname(sess.Request().Host)

	decision, err := h.RouterService.Route(ctx, hostname)
	if err != nil {
		return err
	}

	if decision.Destination == router.DestinationRemoteRegion {
		return h.ProxyService.ForwardToRegion(ctx, sess, decision.RemoteRegionAddress)
	}

	req := sess.Request()

	// The ClickHouse logging middleware seeds an empty tracking record
	// before this handler runs. Populate it now that the route resolved;
	// the retry loop and proxy callbacks fill in the per-attempt instance,
	// timing, and status as the request progresses.
	tracking, ok := proxy.RequestTrackingFromContext(ctx)
	if !ok {
		// Defensive: register.go always wires the ClickHouse logging
		// middleware before this handler, so this branch is unreachable
		// in production. Allocate one so the engine + proxy don't panic
		// if someone reorders middleware.
		//nolint:exhaustruct
		tracking = &proxy.RequestTracking{StartTime: startTime}
		ctx = proxy.WithRequestTracking(ctx, tracking)
	}
	tracking.RequestID = sess.RequestID()
	tracking.DeploymentID = decision.DeploymentID
	tracking.ResourceID = decision.ResourceID
	tracking.ResourceName = decision.ResourceName
	tracking.ResourceKind = decision.ResourceKind
	tracking.WorkspaceID = decision.WorkspaceID
	tracking.ProjectID = decision.ProjectID
	tracking.AppID = decision.AppID
	tracking.EnvironmentID = decision.EnvironmentID

	// Tell the ClickHouse logging middleware which header and query-parameter
	// names carry an API key so it can redact them. KeyAuth supports custom
	// header and query-parameter key delivery; without this the raw key would
	// be persisted verbatim in the request log.
	tracking.RedactedHeaders, tracking.RedactedQueryParams = policies.SecretLocations(decision.Policies)

	// Evaluate policies before forwarding. The edge middleware has already
	// stripped any client-supplied X-Unkey-Principal header; if KeyAuth
	// produces a principal, we set it here for the upstream.
	if len(decision.Policies) > 0 && h.Engine != nil {
		result, evalErr := h.Engine.Evaluate(ctx, sess, req, decision.WorkspaceID, decision.Policies)
		if evalErr != nil {
			return evalErr
		}
		if result.Principal != nil {
			principalJSON, serErr := result.Principal.Marshal()
			if serErr != nil {
				logger.Error("failed to serialize principal", "error", serErr)
			} else {
				req.Header.Set(policies.PrincipalHeader, principalJSON)
			}
		}
	}

	if decision.Destination == router.DestinationStaticArtifact {
		tracking.DirectResponse = true
		return h.serveStatic(ctx, sess, decision.StaticArtifact)
	}

	// Capture the request body for ClickHouse via TeeReader. Bytes flow
	// to the upstream untouched while a copy accumulates in buf, capped
	// at MaxBodyCapture so a multi-GB upload cannot blow the heap. Works
	// for both streaming (gRPC, Connect) and unary requests.
	//
	// With the dial-failure retry loop, a failed first attempt does not
	// consume the body (the proxy never opened a TCP connection), so the
	// successful attempt drains it and the tee captures from that drain.
	// The captured body therefore always reflects what the *serving*
	// instance actually saw.
	if req.Body != nil {
		var buf bytes.Buffer
		req.Body = io.NopCloser(io.TeeReader(req.Body, &zen.LimitedWriter{W: &buf, N: zen.MaxBodyCapture}))
		defer func() {
			if buf.Len() > 0 {
				tracking.RequestBody = buf.Bytes()
			}
		}()
	}

	// Try each candidate instance in turn. We only move to the next
	// instance on dial-phase failures: the proxy never opened a TCP
	// connection, so the request body has not been read and replay is
	// safe. Any other error — mid-stream resets, response timeouts,
	// context cancellation — is returned to the client unchanged, since
	// the upstream may already have started processing the request and a
	// retry would risk double-execute on non-idempotent endpoints.
	//
	// 4xx / 5xx responses from the app are not errors at this layer; they
	// flow back through the proxy's ModifyResponse path and never reach
	// here.
	sawDialFailure := false
	var forwardErr error
	for _, instance := range decision.LocalInstances {
		tracking.InstanceID = instance.ID
		tracking.Address = instance.Address
		tracking.ResourceID = instance.ResourceID
		tracking.ResourceName = instance.ResourceName
		tracking.ResourceKind = string(instance.ResourceKind)

		forwardErr = h.ProxyService.ForwardToInstance(ctx, sess, decision.UpstreamProtocol, instance)
		if forwardErr == nil {
			if sawDialFailure {
				localRequestRetriesTotal.WithLabelValues(retryOutcomeRecovered).Inc()
			}
			return nil
		}
		if !proxy.IsDialError(forwardErr) {
			return forwardErr
		}
		sawDialFailure = true
	}
	if sawDialFailure {
		localRequestRetriesTotal.WithLabelValues(retryOutcomeExhausted).Inc()
	}

	// Every local instance dial-failed (or there were none — shouldn't
	// happen since the router would have returned a remote decision in
	// that case, but treat it the same way). If the router gave us a
	// peer-region standby, fall through to it. The peer redoes its own
	// routing and retry. Without a standby, surface the last dial error.
	if decision.RemoteRegionAddress != "" {
		regionFallbacksTotal.WithLabelValues(decision.RemoteRegionAddress).Inc()
		return h.ProxyService.ForwardToRegion(ctx, sess, decision.RemoteRegionAddress)
	}

	// forwardErr is nil only when the loop never ran, i.e. LocalInstances
	// was empty *and* RemoteRegionAddress was empty. That's an invariant
	// violation: the router should have returned a remote decision instead
	// of a local one with no candidates. Without this guard, returning the
	// zero-value nil here surfaces to the client as a silent empty 200 —
	// fail closed with an explicit 503 so the bug is visible.
	if forwardErr == nil {
		return fault.New("local decision with no instances",
			fault.Code(codes.Frontline.Routing.NoRunningInstances.URN()),
			fault.Internal("router returned DestinationLocalInstance with empty LocalInstances"),
			fault.Public("Service temporarily unavailable"),
		)
	}
	return forwardErr
}

func (h *Handler) serveStatic(ctx context.Context, sess *zen.Session, artifact router.StaticArtifact) error {
	if h.StaticAssets == nil {
		return fault.New("static artifact service is unavailable",
			fault.Code(codes.Frontline.Proxy.ServiceUnavailable.URN()),
			fault.Internal("frontline received a static route without artifact storage configured"),
			fault.Public("Service temporarily unavailable"),
		)
	}

	req := sess.Request()
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		sess.ResponseWriter().Header().Set("Allow", "GET, HEAD")
		sess.ResponseWriter().WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	file, found, err := h.StaticAssets.Resolve(ctx, staticassets.ArtifactRef{
		StorageKey:  artifact.StorageKey,
		Digest:      artifact.Digest,
		SPAFallback: artifact.SPAFallback,
	}, req.URL.EscapedPath())
	if err != nil {
		return fault.Wrap(err,
			fault.Code(codes.Frontline.Proxy.ServiceUnavailable.URN()),
			fault.Internal("unable to resolve immutable static artifact"),
			fault.Public("Service temporarily unavailable"),
		)
	}
	if !found {
		writeStaticError(sess, http.StatusNotFound, "Not Found\n", req.Method == http.MethodHead)
		return nil
	}

	response := sess.ResponseWriter()
	etag := `"` + file.ETag + `"`
	response.Header().Set("Content-Type", file.ContentType)
	response.Header().Set("Cache-Control", file.CacheControl)
	response.Header().Set("ETag", etag)
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Content-Length", strconv.Itoa(len(file.Body)))
	if etagMatches(req.Header.Get("If-None-Match"), etag) {
		response.Header().Del("Content-Length")
		response.WriteHeader(http.StatusNotModified)
		return nil
	}
	response.WriteHeader(http.StatusOK)
	if req.Method == http.MethodHead {
		return nil
	}
	_, err = response.Write(file.Body)
	return err
}

func etagMatches(header string, etag string) bool {
	for value := range strings.SplitSeq(header, ",") {
		candidate := strings.TrimSpace(value)
		if candidate == "*" || candidate == etag || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

func writeStaticError(sess *zen.Session, status int, body string, head bool) {
	response := sess.ResponseWriter()
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Content-Length", strconv.Itoa(len(body)))
	response.WriteHeader(status)
	if !head {
		_, _ = io.WriteString(response, body)
	}
}
