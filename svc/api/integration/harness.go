package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/clickhouse"
	"github.com/unkeyed/unkey/pkg/clock"
	sharedconfig "github.com/unkeyed/unkey/pkg/config"
	"github.com/unkeyed/unkey/pkg/counter"
	"github.com/unkeyed/unkey/pkg/db"
	"github.com/unkeyed/unkey/pkg/mysql/sqlcomment"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/svc/api"
	"github.com/unkeyed/unkey/svc/api/internal/testutil/seed"
	vaulttestutil "github.com/unkeyed/unkey/svc/vault/testutil"
)

// ApiConfig holds configuration for dynamic API container creation
type ApiConfig struct {
	Nodes         int
	MysqlDSN      string
	ClickhouseDSN string
}

// ApiCluster represents a cluster of API containers
type ApiCluster struct {
	Addrs []string
}

// Harness is a test harness for creating and managing a cluster of API nodes
type Harness struct {
	t             *testing.T
	ctx           context.Context
	cancel        context.CancelFunc
	instanceAddrs []string
	Seed          *seed.Seeder
	Clock         *clock.TestClock
	dbDSN         string
	chDSN         string
	DB            db.Database
	CH            clickhouse.ClickHouse
	apiCluster    *ApiCluster
	// counter is a shared in-memory counter used by all API nodes in the
	// cluster. Using an in-process counter instead of real Redis ensures
	// replay workers sync in microseconds, keeping up with simulated-time
	// load tests that run many orders of magnitude faster than real time.
	counter counter.Counter
}

// Config contains configuration options for the test harness
type Config struct {
	// NumNodes is the number of API nodes to create in the cluster
	NumNodes int

	// TestClock, when set, is shared by every API node in the harness. Tests
	// that advance simulated time must use this instead of a local clock so
	// request handlers and background services observe the same time.
	TestClock *clock.TestClock
}

// New creates a new cluster test harness
func New(t *testing.T, config Config) *Harness {
	t.Helper()

	require.Greater(t, config.NumNodes, 0)
	ctx, cancel := context.WithCancel(context.Background())

	// Spin up MySQL and ClickHouse once per harness; all in-process API
	// nodes share these.
	mysqlCfg := containers.MySQL(t)
	chCfg := containers.ClickHouse(t)

	ch, err := clickhouse.New(clickhouse.Config{
		URL: chCfg.DSN,
	})
	require.NoError(t, err)

	db, err := db.New(db.Config{
		PrimaryDSN:  mysqlCfg.DSN,
		ReadOnlyDSN: "",
		Tags:        sqlcomment.Disabled(),
	})
	require.NoError(t, err)

	h := &Harness{
		t:             t,
		ctx:           ctx,
		cancel:        cancel,
		instanceAddrs: []string{},
		Seed:          seed.New(t, db, nil),
		Clock:         config.TestClock,
		dbDSN:         mysqlCfg.DSN,
		chDSN:         chCfg.DSN,
		DB:            db,
		CH:            ch,
		apiCluster:    nil, // Will be set later
		counter:       counter.NewMemory(),
	}
	t.Cleanup(func() {
		h.cancel()
		require.NoError(t, h.counter.Close())
		require.NoError(t, h.DB.Close())
		require.NoError(t, h.CH.Close())
	})

	h.Seed.Seed(ctx)

	// Create dynamic API container cluster
	cluster := h.RunAPI(ApiConfig{
		Nodes:         config.NumNodes,
		MysqlDSN:      mysqlCfg.DSN,
		ClickhouseDSN: chCfg.DSN,
	})
	h.apiCluster = cluster
	h.instanceAddrs = cluster.Addrs
	return h
}

func (h *Harness) Resources() seed.Resources {
	return h.Seed.Resources
}

// RunAPI creates a cluster of API containers for chaos testing
func (h *Harness) RunAPI(config ApiConfig) *ApiCluster {
	cluster := &ApiCluster{
		Addrs: make([]string, config.Nodes),
	}
	type runningNode struct {
		id       int
		cancel   context.CancelFunc
		shutdown <-chan error
	}
	nodes := make([]runningNode, 0, config.Nodes)
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			for _, node := range nodes {
				node.cancel()
			}

			timer := time.NewTimer(90 * time.Second)
			defer timer.Stop()
			for _, node := range nodes {
				select {
				case err := <-node.shutdown:
					if err != nil {
						h.t.Errorf("API server %d shutdown failed: %v", node.id, err)
					}
				case <-timer.C:
					h.t.Errorf("API server shutdown timeout: nodes did not stop within 90 seconds")
					return
				}
			}
		})
	}

	// Start each API node as a goroutine
	for i := 0; i < config.Nodes; i++ {
		nodeID := i
		// Create ephemeral listener
		ln, err := net.Listen("tcp", ":0") //nolint: gosec
		require.NoError(h.t, err, "Failed to create ephemeral listener")

		cluster.Addrs[i] = fmt.Sprintf("http://%s", ln.Addr().String())

		// Create API config for this node using host connections
		testVault := vaulttestutil.StartTestVaultWithMemory(h.t)
		apiClock := clock.Clock(clock.New())
		if h.Clock != nil {
			apiClock = h.Clock
		}

		apiConfig := api.Config{
			HttpPort:   7070,
			Platform:   "test",
			Image:      "test",
			RedisURL:   "", // Ignored: Test.Counter overrides the backend.
			Region:     "test",
			InstanceID: fmt.Sprintf("test-node-%d", i),
			Clock:      apiClock,
			Test: api.TestConfig{
				Enabled:  true,
				Counter:  h.counter,
				Listener: ln,
			},
			TLSConfig:          nil,
			MaxRequestBodySize: 0,
			Database: sharedconfig.DatabaseConfig{
				Primary:         h.dbDSN,
				ReadonlyReplica: "",
			},
			ClickHouse: api.ClickHouseConfig{
				URL:          h.chDSN,
				AnalyticsURL: "",
			},
			Observability: sharedconfig.Observability{
				Tracing: nil,

				// our tests were drowning in logs, so we disable them ehre
				Logging: &sharedconfig.LoggingConfig{
					SampleRate:    0,
					SlowThreshold: time.Hour,
				},
				Metrics: &sharedconfig.MetricsConfig{
					PrometheusPort: 0,
				},
			},
			TLS: sharedconfig.TLS{
				Disabled: true,
				CertFile: "",
				KeyFile:  "",
			},
			Vault: sharedconfig.VaultConfig{
				URL:   testVault.URL,
				Token: testVault.Token,
			},
			Control: sharedconfig.ControlConfig{
				URL:   "http://control:7091",
				Token: "your-local-dev-key",
			},
			Pprof: &sharedconfig.PprofConfig{
				Username: "unkey",
				Password: "password",
				Port:     0,
			},
			Auth: api.AuthConfigs{
				api.PortalSessionAuthConfig{},
				api.RootKeyAuthConfig{Enabled: nil},
			},
			PortalBaseURL: "https://portal.test.local",
		}

		// Start API server in goroutine
		ctx, cancel := context.WithCancel(context.Background())

		// Separate startup and shutdown channels let readiness checks observe an
		// early process failure without consuming the completion signal cleanup
		// must await.
		startupResult := make(chan error, 1)
		shutdownResult := make(chan error, 1)

		go func(nodeID int, cfg api.Config) {
			var runErr error
			defer func() {
				if r := recover(); r != nil {
					h.t.Logf("API server %d panicked: %v", nodeID, r)
					runErr = fmt.Errorf("panic: %v", r)
				}
				if runErr == nil && ctx.Err() == nil {
					runErr = fmt.Errorf("API server %d stopped before cancellation", nodeID)
				}
				if runErr != nil && ctx.Err() == nil {
					select {
					case startupResult <- runErr:
					default:
					}
				}
				shutdownResult <- runErr
			}()

			runErr = api.Run(ctx, cfg)
			if runErr != nil && ctx.Err() == nil {
				h.t.Logf("API server %d failed: %v", nodeID, runErr)
			}
		}(nodeID, apiConfig)

		nodes = append(nodes, runningNode{
			id:       nodeID,
			cancel:   cancel,
			shutdown: shutdownResult,
		})
		// Register after the node's vault cleanup so this shared once-guarded
		// callback runs first, stops every node, and only then releases vaults.
		h.t.Cleanup(shutdown)

		// Wait for server to start
		maxAttempts := 30
		healthURL := fmt.Sprintf("http://%s/v2/liveness", ln.Addr().String())
		for attempt := range maxAttempts {
			select {
			case err := <-startupResult:
				require.NoError(h.t, err, "API server %d startup failed", nodeID)
			default:
			}
			//nolint:gosec // Health check URL is constructed from controlled Docker container address
			resp, err := http.Get(healthURL)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					h.t.Logf("API server %d started on %s", i, ln.Addr().String())
					break
				}
			}
			if attempt == maxAttempts-1 {
				require.FailNow(h.t, "API server failed to start", "node %d: %v", nodeID, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	return cluster
}

// GetClusterAddrs returns the addresses of all API containers
func (h *Harness) GetClusterAddrs() []string {
	if h.apiCluster == nil {
		return []string{}
	}
	return h.apiCluster.Addrs
}
