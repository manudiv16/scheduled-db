package internal

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"scheduled-db/internal/api"
	"scheduled-db/internal/logger"
	"scheduled-db/internal/metrics"
	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"

	"github.com/hashicorp/raft"
	"github.com/manudiv16/pkgcluster"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
)

type App struct {
	store              *store.Store
	statusTracker      *store.StatusTracker
	executionManager   *slots.ExecutionManager
	slotQueue          *slots.PersistentSlotQueue
	worker             *slots.Worker
	slotEvictor        *slots.SlotEvictor
	httpServer         *http.Server
	metricsServer      *http.Server
	prometheusExporter *prometheus.Exporter
	discoveryManager   *pkgcluster.Manager
	nodeID             string
	raftAdvertise      string
	useDiscovery       bool
	bootstrapExpect    int
	peerTracker        *peerTracker
	shutdownSignal     chan os.Signal
}

type Config struct {
	DataDir                   string
	RaftBind                  string
	RaftAdvertise             string
	HTTPBind                  string
	NodeID                    string
	Peers                     []string
	SlotGap                   time.Duration
	Topologies                []pkgcluster.Topology
	ExecutionTimeout          time.Duration
	InProgressTimeout         time.Duration
	MaxExecutionAttempts      int
	HistoryRetention          time.Duration
	HealthFailureThreshold    float64
	QueueMemoryLimit          int64
	QueueJobLimit             int64
	EnableColdSpilling        bool
	ColdSpillingHotWindow     time.Duration
	ColdSpillingCheckInterval time.Duration
	TimingWheelConfigs        []slots.WheelLevelConfig
	BootstrapExpect           int

	// Security configuration
	HTTPReadTimeout          time.Duration
	HTTPReadHeaderTimeout    time.Duration
	HTTPWriteTimeout         time.Duration
	HTTPIdleTimeout          time.Duration
	MaxRequestBodySize       int64
	AuthToken                string
}

// NewApp creates a new application instance.
//
// Discovery topologies are provided via config.Topologies. Each topology
// carries a strategy type and config map but no callbacks — NewApp fills
// in the Connect/Disconnect/ListNodes closures after creating the store,
// so that the discovery layer never depends on Raft or store types.
func NewApp(config *Config) (*App, error) {
	// Initialize metrics system
	ctx := context.Background()
	metricsConfig := &metrics.Config{
		ServiceName:    "scheduled-db",
		ServiceVersion: "1.0.0",
		NodeID:         config.NodeID,
		Environment:    "production",
		MetricsPort:    9090,
		MetricsPath:    "/metrics",
	}

	// Setup metrics with OTLP and Prometheus exporters
	_, cleanup, prometheusExporter, err := metrics.InitializeWithOTLP(ctx, metricsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %v", err)
	}

	// Build a Raft ServerAddressProvider from topology config when a
	// Kubernetes or DNS strategy is used (for StatefulSet DNS resolution).
	var addrProvider raft.ServerAddressProvider
	for _, t := range config.Topologies {
		if t.Strategy == pkgcluster.StrategyKubernetes ||
			t.Strategy == pkgcluster.StrategyKubernetesDNS ||
			t.Strategy == pkgcluster.StrategyKubernetesDNSSRV {
			serviceName := pkgcluster.StringConfigFromMap(t.Config, "service_name", "scheduled-db")
			namespace := pkgcluster.StringConfigFromMap(t.Config, "namespace", "default")
			domain := pkgcluster.StringConfigFromMap(t.Config, "cluster_domain", "cluster.local")
			addrProvider = store.NewRaftAddressProvider(serviceName, namespace, domain, 7000)
			break
		}
	}

	// Create store with Raft (no bootstrap yet — bootstrap-expect will handle it)
	jobStore, err := store.NewStoreWithColdSpilling(config.DataDir, config.RaftBind, config.RaftAdvertise, config.NodeID, config.Peers, config.EnableColdSpilling, addrProvider)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create store: %v", err)
	}

	// Build discovery topologies with callbacks wired to the store.
	// When bootstrap-expect is enabled, the Connect callback feeds a peer
	// tracker so the app can defer bootstrap until enough peers are known.
	var peerTracker *peerTracker
	if config.BootstrapExpect > 0 {
		peerTracker = newPeerTracker(config.BootstrapExpect)
	}

	// Build the local Raft advertise address once for self-filtering in callbacks.
	localRaftAddr := config.RaftAdvertise

	topologies := make([]pkgcluster.Topology, len(config.Topologies))
	for i, t := range config.Topologies {
		// Capture t by value for the closure.
		topo := t

		// Build the address discovery → Raft address mapping from topology config.
		raftPort := pkgcluster.IntConfigFromMap(topo.Config, "port", 7000)

		topo.Connect = func(ctx context.Context, addr string) error {
			// Skip self — discovery may report our own address.
			if addr == localRaftAddr {
				return nil
			}

			logger.Debug("discovery: connecting peer %s", addr)

			// Feed the peer tracker when bootstrap-expect is active.
			if peerTracker != nil {
				peerTracker.add(addr)
			}

			// Try to add the peer to Raft. Before bootstrap this will fail
			// (not leader) — that's expected; the bootstrap flow handles it.
			if err := jobStore.AddPeer(derivePeerID(addr), addr); err != nil {
				logger.Debug("discovery: add peer %s deferred (not leader yet): %v", addr, err)
			}
			return nil
		}
		topo.Disconnect = func(ctx context.Context, addr string) error {
			logger.Debug("discovery: disconnecting peer %s", addr)
			return jobStore.RemovePeer(derivePeerID(addr))
		}
		topo.ListNodes = func(ctx context.Context) ([]string, error) {
			servers, err := jobStore.GetClusterConfiguration()
			if err != nil {
				return nil, err
			}
			addrs := make([]string, 0, len(servers))
			for _, s := range servers {
				addrs = append(addrs, string(s.Address))
			}
			return addrs, nil
		}

		// Ensure port in config is used for raft port when producing addresses.
		topo.Config["port"] = raftPort

		topologies[i] = topo
	}

	// Create discovery manager only if topologies are configured.
	var discoveryManager *pkgcluster.Manager
	useDiscovery := len(topologies) > 0
	if useDiscovery {
		discoveryManager = pkgcluster.NewManager(topologies...)
	}

	// Create slot queue
	var slotQueue *slots.PersistentSlotQueue
	if len(config.TimingWheelConfigs) > 0 {
		slotQueue = slots.NewPersistentSlotQueueWithConfig(config.SlotGap, jobStore, config.TimingWheelConfigs)
	} else {
		slotQueue = slots.NewPersistentSlotQueue(config.SlotGap, jobStore)
	}

	// Create status tracker
	statusTracker := store.NewStatusTracker(jobStore)

	// Start pruning goroutine for execution history
	statusTracker.StartPruning(config.HistoryRetention)

	// Create execution manager
	executionManager := slots.NewExecutionManager(
		statusTracker,
		jobStore,
		config.NodeID,
		config.ExecutionTimeout,
		config.MaxExecutionAttempts,
	)

	// Create worker
	worker := slots.NewWorker(slotQueue, jobStore, executionManager, config.InProgressTimeout)

	var slotEvictor *slots.SlotEvictor
	if config.EnableColdSpilling {
		slotEvictor = slots.NewSlotEvictorWithWheel(jobStore, slots.SlotEvictionConfig{
			Enabled:       true,
			HotWindow:     config.ColdSpillingHotWindow,
			CheckInterval: config.ColdSpillingCheckInterval,
		}, slotQueue.GetWheel())
	}

	// Pass HTTP bind info to store
	jobStore.SetHTTPBind(config.HTTPBind)

	// Initialize capacity tracking components
	sizeCalculator := slots.NewSizeCalculator()
	memoryTracker := slots.NewMemoryTracker(jobStore, config.QueueMemoryLimit)
	jobCounter := slots.NewJobCounter(jobStore, config.QueueJobLimit)
	limitManager := slots.NewLimitManager(memoryTracker, jobCounter, sizeCalculator)

	// Setup HTTP API
	handlers := api.NewHandlers(jobStore, executionManager, limitManager, config.HealthFailureThreshold)
	router := api.NewRouter(handlers, config.MaxRequestBodySize, config.AuthToken)

	httpServer := &http.Server{
		Addr:              config.HTTPBind,
		Handler:           router,
		ReadTimeout:       config.HTTPReadTimeout,
		ReadHeaderTimeout: config.HTTPReadHeaderTimeout,
		WriteTimeout:      config.HTTPWriteTimeout,
		IdleTimeout:       config.HTTPIdleTimeout,
	}
	// Setup metrics server
	metricsRouter := http.NewServeMux()
	metricsRouter.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", metricsConfig.MetricsPort),
		Handler: metricsRouter,
	}

	// Setup graceful shutdown signal handling
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGTERM, syscall.SIGINT)

	app := &App{
		store:              jobStore,
		statusTracker:      statusTracker,
		executionManager:   executionManager,
		slotQueue:          slotQueue,
		worker:             worker,
		slotEvictor:        slotEvictor,
		httpServer:         httpServer,
		metricsServer:      metricsServer,
		prometheusExporter: prometheusExporter,
		discoveryManager:   discoveryManager,
		nodeID:             config.NodeID,
		raftAdvertise:      config.RaftAdvertise,
		useDiscovery:       useDiscovery,
		bootstrapExpect:    config.BootstrapExpect,
		peerTracker:        peerTracker,
		shutdownSignal:     shutdownSignal,
	}

	// Setup event handler for job changes
	jobStore.SetEventHandler(func(event string, job *store.Job) {
		if jobStore.IsLeader() {
			switch event {
			case "created":
				if job != nil {
					slotQueue.AddJob(job)
					logger.Debug("added job %s to slot queue", job.ID)
					if metrics.GlobalJobInstrumentation != nil {
						metrics.GlobalJobInstrumentation.RecordJobCreated(context.Background(), job)
					}
				}
			case "deleted":
				if job != nil {
					slotQueue.RemoveJob(job.ID)
					logger.Debug("removed job %s from slot queue", job.ID)
					if metrics.GlobalJobInstrumentation != nil {
						metrics.GlobalJobInstrumentation.RecordJobDeleted(context.Background(), job)
					}
				}
			}
		}
	})

	// Setup leadership change handler for all nodes
	go app.monitorLeadership()
	go app.handleGracefulShutdown()

	return app, nil
}

func (a *App) Start() error {
	logger.Info("starting node %s", a.nodeID)

	// Start discovery manager only if enabled
	if a.useDiscovery {
		ctx := context.Background()
		a.discoveryManager.Start(ctx)
	}

	// Bootstrap-expect: wait for enough peers and form the cluster.
	if a.bootstrapExpect > 0 {
		if err := a.waitForBootstrap(); err != nil {
			return fmt.Errorf("bootstrap-expect: %v", err)
		}
	}

	// If this node becomes leader, load jobs and start worker
	if a.store.IsLeader() {
		a.becomeLeader()
	}

	// Start HTTP server in background
	go func() {
		logger.Info("starting HTTP server on %s", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error: %v", err)
		}
	}()

	// Start metrics server in background
	go func() {
		logger.Info("starting metrics server on %s", a.metricsServer.Addr)
		if err := a.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Metrics server error: %v", err)
		}
	}()

	return nil
}

func (a *App) Stop() error {
	logger.Info("stopping application...")

	done := make(chan bool, 1)

	go func() {
		defer func() { done <- true }()

		// Stop discovery manager if enabled
		if a.useDiscovery {
			logger.Info("stopping discovery manager...")
			a.discoveryManager.Stop()
		}

		// Stop pruning goroutine
		logger.Info("stopping execution history pruning...")
		a.statusTracker.StopPruning()

		// Stop worker
		logger.Info("stopping worker...")
		a.worker.Stop()

		// Force close HTTP server
		logger.Info("force closing HTTP server...")
		if err := a.httpServer.Close(); err != nil {
			logger.Error("error force closing HTTP server: %v", err)
		}

		// Force close metrics server
		logger.Info("force closing metrics server...")
		if err := a.metricsServer.Close(); err != nil {
			logger.Error("error force closing metrics server: %v", err)
		}

		// Close Raft store with timeout
		logger.Info("stopping Raft store...")
		storeDone := make(chan error, 1)
		go func() {
			storeDone <- a.store.Close()
		}()

		select {
		case err := <-storeDone:
			if err != nil {
				logger.Error("error closing store: %v", err)
			}
		case <-time.After(10 * time.Second):
			logger.Warn("raft store shutdown timeout, forcing exit...")
		}
	}()

	select {
	case <-done:
		logger.Info("all components stopped successfully")
	case <-time.After(30 * time.Second):
		logger.Warn("shutdown timeout reached, forcing exit")
	}

	return nil
}

// handleGracefulShutdown implements graceful leader resignation on SIGTERM
func (a *App) handleGracefulShutdown() {
	sig := <-a.shutdownSignal
	logger.Info("[GRACEFUL SHUTDOWN] received signal: %v", sig)

	if a.store.IsLeader() {
		logger.Info("[GRACEFUL SHUTDOWN] I am the leader - performing graceful resignation")

		logger.Info("[GRACEFUL SHUTDOWN] stopping worker to prevent new job processing")
		a.worker.Stop()

		logger.Info("[GRACEFUL SHUTDOWN] removing myself from cluster configuration")
		if err := a.store.RemovePeer(a.nodeID); err != nil {
			logger.Error("[GRACEFUL SHUTDOWN] failed to remove self from cluster: %v", err)
		} else {
			logger.Info("[GRACEFUL SHUTDOWN] successfully removed self from cluster")
		}

		logger.Info("[GRACEFUL SHUTDOWN] waiting for followers to start election...")
		time.Sleep(2 * time.Second)

		logger.Info("[GRACEFUL SHUTDOWN] stepping down from leadership")
		future := a.store.GetRaft().LeadershipTransfer()
		if err := future.Error(); err != nil {
			logger.Error("[GRACEFUL SHUTDOWN] leadership transfer failed: %v", err)
		} else {
			logger.Info("[GRACEFUL SHUTDOWN] leadership transfer initiated")
		}

		time.Sleep(1 * time.Second)
	} else {
		logger.Info("[GRACEFUL SHUTDOWN] I am a follower - performing normal shutdown")
	}

	logger.Info("[GRACEFUL SHUTDOWN] performing final shutdown")
	if err := a.Stop(); err != nil {
		logger.Error("[GRACEFUL SHUTDOWN] error during shutdown: %v", err)
		os.Exit(1)
	}

	logger.Info("[GRACEFUL SHUTDOWN] shutdown completed successfully")
	os.Exit(0)
}

func (a *App) monitorLeadership() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	wasLeader := a.store.IsLeader()
	logger.Debug("starting leadership monitoring, initial state: isLeader=%v", wasLeader)
	noLeaderCount := 0

	for range ticker.C {
		isLeader := a.store.IsLeader()
		currentLeader := a.store.GetLeader()

		if !wasLeader && isLeader {
			logger.ClusterInfo("node %s became leader", a.nodeID)
			a.becomeLeader()
			noLeaderCount = 0
		} else if wasLeader && !isLeader {
			logger.ClusterWarn("node %s lost leadership, current leader: %s", a.nodeID, currentLeader)
			a.loseLeadership()
		}

		if currentLeader == "" {
			noLeaderCount++
			if noLeaderCount > 60 {
				servers, err := a.store.GetClusterConfiguration()
				if err == nil && len(servers) == 0 {
					logger.ClusterWarn("no leader for 60 seconds and empty cluster, attempting auto-bootstrap")
					if err := a.attemptAutoBootstrap(); err != nil {
						logger.ClusterError("auto-bootstrap failed: %v", err)
					} else {
						logger.ClusterInfo("auto-bootstrap successful")
						noLeaderCount = 0
					}
				}
			}
		} else {
			noLeaderCount = 0
		}

		if ticker := time.Now().Unix(); ticker%30 == 0 {
			servers, err := a.store.GetClusterConfiguration()
			var clusterInfo string
			if err != nil {
				clusterInfo = fmt.Sprintf("error: %v", err)
			} else {
				serverList := make([]string, len(servers))
				for i, server := range servers {
					serverList[i] = fmt.Sprintf("%s@%s", server.ID, server.Address)
				}
				clusterInfo = fmt.Sprintf("servers=[%s]", strings.Join(serverList, ", "))
			}
			raftState := a.store.GetRaftState()
			logger.Debug("node %s status: isLeader=%v, currentLeader=%s, raftState=%s, cluster=%s",
				a.nodeID, isLeader, currentLeader, raftState, clusterInfo)
		}

		wasLeader = isLeader
	}
}

func (a *App) becomeLeader() {
	logger.ClusterInfo("node %s becoming leader - starting worker", a.nodeID)
	a.worker.Start()
	if a.slotEvictor != nil {
		a.slotEvictor.Start()
	}
	logger.Info("🎯 CLUSTER READY: Node %s is leader, cluster fully operational", a.nodeID)
}

func (a *App) loseLeadership() {
	logger.ClusterInfo("node %s losing leadership - stopping worker", a.nodeID)
	a.worker.Stop()
	if a.slotEvictor != nil {
		a.slotEvictor.Stop()
	}
	logger.ClusterInfo("node %s worker stopped due to leadership loss", a.nodeID)
}

func (a *App) attemptAutoBootstrap() error {
	logger.Debug("[LEADERSHIP DEBUG] node %s attempting auto-bootstrap as single-node cluster", a.nodeID)
	return a.store.ForceBootstrap(a.nodeID)
}

// ---------------------------------------------------------------------------
// Bootstrap-expect helpers
// ---------------------------------------------------------------------------

// peerTracker tracks discovered peer addresses and signals when a threshold
// is reached. It is goroutine-safe.
type peerTracker struct {
	mu        sync.Mutex
	addresses map[string]struct{}
	threshold int
	done      chan struct{}
	closed    bool
}

func newPeerTracker(threshold int) *peerTracker {
	return &peerTracker{
		addresses: make(map[string]struct{}),
		threshold: threshold,
		done:      make(chan struct{}),
	}
}

// add records a discovered peer address. If the threshold is met for the
// first time, the done channel is closed.
func (pt *peerTracker) add(addr string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.closed {
		return
	}
	pt.addresses[addr] = struct{}{}
	if len(pt.addresses) >= pt.threshold {
		pt.closed = true
		close(pt.done)
	}
}

// wait blocks until the threshold is reached or the context is cancelled.
// It returns the list of discovered peer addresses.
func (pt *peerTracker) wait(ctx context.Context) []string {
	select {
	case <-pt.done:
	case <-ctx.Done():
	}
	pt.mu.Lock()
	defer pt.mu.Unlock()
	addrs := make([]string, 0, len(pt.addresses))
	for addr := range pt.addresses {
		addrs = append(addrs, addr)
	}
	return addrs
}

// count returns the current number of tracked peer addresses.
func (pt *peerTracker) count() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return len(pt.addresses)
}

// list returns a copy of the tracked addresses.
func (pt *peerTracker) list() []string {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	addrs := make([]string, 0, len(pt.addresses))
	for addr := range pt.addresses {
		addrs = append(addrs, addr)
	}
	return addrs
}

// derivePeerID extracts a Raft server ID from a peer address.
//
// For DNS names (e.g., "scheduled-db-1.scheduled-db.default.svc:7000"), the
// first DNS label is the StatefulSet pod name, which matches the node ID.
// For plain hostnames or IPs, the host portion is returned as-is.
//
// The caller MUST ensure the returned ID matches the peer's --node-id for
// Raft to function correctly. This works naturally with Kubernetes DNS and
// hostname discovery modes. In IP mode the ID will be an IP literal, which
// will NOT match the node's actual ID — use DNS/hostname mode or configure
// an explicit mapping.
func derivePeerID(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	// If DNS name, first label is the pod name (StatefulSet convention).
	if idx := strings.IndexByte(host, '.'); idx > 0 {
		return host[:idx]
	}
	return host
}

// waitForBootstrap implements the bootstrap-expect flow. It waits for enough
// peers to be discovered, then either initiates the bootstrap (if this node
// is the designated initiator) or waits for the cluster to form.
func (a *App) waitForBootstrap() error {
	logger.ClusterInfo("bootstrap-expect=%d: waiting for peers to be discovered", a.bootstrapExpect)

	if a.peerTracker == nil {
		return fmt.Errorf("bootstrap-expect enabled but peer tracker not initialized")
	}

	// Wait for threshold with a generous timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	peers := a.peerTracker.wait(ctx)
	logger.ClusterInfo("bootstrap-expect: discovered %d peer(s) (expected %d)", len(peers), a.bootstrapExpect-1)

	// Determine if this node should initiate the bootstrap.
	if a.isBootstrapInitiator(peers) {
		logger.ClusterInfo("designated bootstrap initiator for %d-node cluster", a.bootstrapExpect)

		// Bootstrap self as single-node cluster.
		if err := a.store.ForceBootstrap(a.nodeID); err != nil {
			logger.Debug("bootstrap initiator: self-bootstrap result: %v", err)
		}

		// Wait to become leader.
		if err := a.store.WaitForLeader(10 * time.Second); err != nil {
			logger.Warn("bootstrap initiator: not leader after bootstrap, will retry")
		}

		// Add discovered peers to the cluster.
		if a.store.IsLeader() {
			for _, addr := range peers {
				peerID := derivePeerID(addr)
				logger.ClusterInfo("adding peer %s at %s", peerID, addr)
				if err := a.store.AddPeer(peerID, addr); err != nil {
					logger.Warn("failed to add peer %s: %v", peerID, err)
				}
			}
			servers, _ := a.store.GetClusterConfiguration()
			logger.ClusterInfo("bootstrap complete: cluster has %d server(s)", len(servers))
		}
	} else {
		logger.ClusterInfo("waiting for bootstrap initiator to form cluster...")
		if err := a.store.WaitForLeader(60 * time.Second); err != nil {
			logger.Warn("bootstrap-expect: timeout waiting for leader, will proceed with discovery")
		}
	}

	return nil
}

// isBootstrapInitiator returns true if this node should initiate the cluster
// bootstrap. The node with the smallest node ID (compared against derived
// peer IDs) is the designated initiator, ensuring exactly one node bootstraps.
func (a *App) isBootstrapInitiator(peers []string) bool {
	for _, addr := range peers {
		if derivePeerID(addr) < a.nodeID {
			return false
		}
	}
	return true
}
