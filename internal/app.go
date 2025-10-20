package internal

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"scheduled-db/internal/api"
	"scheduled-db/internal/discovery"
	"scheduled-db/internal/logger"
	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"
)

type App struct {
	store            *store.Store
	slotQueue        *slots.PersistentSlotQueue
	worker           *slots.Worker
	httpServer       *http.Server
	discoveryManager *discovery.DiscoveryManager
	nodeID           string
	useDiscovery     bool
	shutdownSignal   chan os.Signal
}

type Config struct {
	DataDir         string
	RaftBind        string
	RaftAdvertise   string
	HTTPBind        string
	NodeID          string
	Peers           []string
	SlotGap         time.Duration
	DiscoveryConfig discovery.DiscoveryConfig
}

func NewApp(config *Config) (*App, error) {
	// Create store with Raft (start with configured peers, discovery will handle dynamic joining)
	jobStore, err := store.NewStore(config.DataDir, config.RaftBind, config.RaftAdvertise, config.NodeID, config.Peers)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %v", err)
	}

	// Wait for leader election (shorter timeout for nodes with peers)
	timeout := 30 * time.Second
	if len(config.Peers) > 0 {
		timeout = 5 * time.Second // Shorter timeout for joining nodes
	}

	if err := jobStore.WaitForLeader(timeout); err != nil {
		if len(config.Peers) == 0 {
			logger.Warn("no leader found yet, will attempt join after discovery starts")
		} else {
			return nil, fmt.Errorf("failed to wait for leader: %v", err)
		}
	}

	shutdownCallback := func() error {
		logger.Error("split-brain detected, shutting down node")
		// This will be called by split-brain detection to shutdown the app
		go func() {
			time.Sleep(100 * time.Millisecond) // Small delay to allow log to be written
			os.Exit(42)                        // Exit code for split-brain prevention
		}()
		return nil
	}

	// Create discovery manager only if discovery is enabled
	var discoveryManager *discovery.DiscoveryManager
	useDiscovery := config.DiscoveryConfig.Strategy != ""
	if useDiscovery {
		discoveryManager, err = discovery.NewDiscoveryManager(config.DiscoveryConfig, jobStore, shutdownCallback)
		if err != nil {
			return nil, fmt.Errorf("failed to create discovery manager: %v", err)
		}
	}

	// Create slot queue
	slotQueue := slots.NewPersistentSlotQueue(config.SlotGap, jobStore)

	// Create worker
	worker := slots.NewWorker(slotQueue, jobStore)

	// Pass HTTP bind info to store
	jobStore.SetHTTPBind(config.HTTPBind)

	// Setup HTTP API
	handlers := api.NewHandlers(jobStore)
	router := api.NewRouter(handlers)

	httpServer := &http.Server{
		Addr:    config.HTTPBind,
		Handler: router,
	}

	// Setup graceful shutdown signal handling
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGTERM, syscall.SIGINT)

	app := &App{
		store:            jobStore,
		slotQueue:        slotQueue,
		worker:           worker,
		httpServer:       httpServer,
		discoveryManager: discoveryManager,
		nodeID:           config.NodeID,
		useDiscovery:     useDiscovery,
		shutdownSignal:   shutdownSignal,
	}

	// Setup event handler for job changes
	jobStore.SetEventHandler(func(event string, job *store.Job) {
		if jobStore.IsLeader() {
			switch event {
			case "created":
				if job != nil {
					slotQueue.AddJob(job)
					logger.Debug("added job %s to slot queue", job.ID)
				}
			case "deleted":
				if job != nil {
					slotQueue.RemoveJob(job.ID)
					logger.Debug("removed job %s from slot queue", job.ID)
				}
			}
		}
	})

	// Setup leadership change handler for all nodes (needed for high availability)
	go app.monitorLeadership()

	// Setup graceful shutdown handler
	go app.handleGracefulShutdown()

	return app, nil
}

func (a *App) Start() error {
	logger.Info("starting node %s", a.nodeID)

	// Start discovery manager only if enabled
	if a.useDiscovery {
		if err := a.discoveryManager.Start(); err != nil {
			return fmt.Errorf("failed to start discovery manager: %v", err)
		}

		if a.store.GetLeader() == "" {
			logger.Debug("no leader found, discovery will help coordinate cluster join")
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

	return nil
}

func (a *App) Stop() error {
	logger.Info("stopping application...")

	// Use shorter timeouts and force shutdown if needed
	done := make(chan bool, 1)

	go func() {
		defer func() { done <- true }()

		// Stop discovery manager if enabled
		if a.useDiscovery {
			logger.Info("stopping discovery manager...")
			discoveryDone := make(chan error, 1)
			go func() {
				discoveryDone <- a.discoveryManager.Stop()
			}()

			select {
			case err := <-discoveryDone:
				if err != nil {
					logger.Error("error stopping discovery manager: %v", err)
				}
			case <-time.After(10 * time.Second):
				logger.Warn("discovery manager shutdown timeout, continuing...")
			}
		}

		// Stop worker
		logger.Info("stopping worker...")
		a.worker.Stop()

		// Force close HTTP server (don't wait for graceful)
		logger.Info("force closing HTTP server...")
		if err := a.httpServer.Close(); err != nil {
			logger.Error("error force closing HTTP server: %v", err)
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

	// Force exit after 10 seconds total
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

		// Step 1: Stop accepting new work
		logger.Info("[GRACEFUL SHUTDOWN] stopping worker to prevent new job processing")
		a.worker.Stop()

		// Step 2: Remove myself from cluster configuration
		logger.Info("[GRACEFUL SHUTDOWN] removing myself from cluster configuration")
		if err := a.store.RemovePeer(a.nodeID); err != nil {
			logger.Error("[GRACEFUL SHUTDOWN] failed to remove self from cluster: %v", err)
		} else {
			logger.Info("[GRACEFUL SHUTDOWN] successfully removed self from cluster")
		}

		// Step 3: Wait a bit for followers to detect leader loss and start election
		logger.Info("[GRACEFUL SHUTDOWN] waiting for followers to start election...")
		time.Sleep(2 * time.Second)

		// Step 4: Step down from leadership
		logger.Info("[GRACEFUL SHUTDOWN] stepping down from leadership")
		future := a.store.GetRaft().LeadershipTransfer()
		if err := future.Error(); err != nil {
			logger.Error("[GRACEFUL SHUTDOWN] leadership transfer failed: %v", err)
		} else {
			logger.Info("[GRACEFUL SHUTDOWN] leadership transfer initiated")
		}

		// Step 5: Wait a bit more for transition to complete
		time.Sleep(1 * time.Second)
	} else {
		logger.Info("[GRACEFUL SHUTDOWN] I am a follower - performing normal shutdown")
	}

	// Final shutdown
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
			// Became leader
			logger.ClusterInfo("node %s became leader", a.nodeID)
			a.becomeLeader()
			noLeaderCount = 0
		} else if wasLeader && !isLeader {
			// Lost leadership
			logger.ClusterWarn("node %s lost leadership, current leader: %s", a.nodeID, currentLeader)
			a.loseLeadership()
		}

		// Check for orphaned cluster situation
		if currentLeader == "" {
			noLeaderCount++
			if noLeaderCount > 60 { // Wait 60 seconds without leader
				servers, err := a.store.GetClusterConfiguration()
				if err == nil && len(servers) == 0 {
					// Empty cluster - attempt auto-bootstrap
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

		// Periodic status log (every 30 seconds)
		if ticker := time.Now().Unix(); ticker%30 == 0 {
			// Get cluster configuration for debugging
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

	// Start worker (slots are already loaded from persistent store)
	a.worker.Start()

	logger.Info("🎯 CLUSTER READY: Node %s is leader, cluster fully operational", a.nodeID)
}

func (a *App) loseLeadership() {
	logger.ClusterInfo("node %s losing leadership - stopping worker", a.nodeID)
	a.worker.Stop()
	logger.ClusterInfo("node %s worker stopped due to leadership loss", a.nodeID)
}

func (a *App) attemptAutoBootstrap() error {
	logger.Debug("[LEADERSHIP DEBUG] node %s attempting auto-bootstrap as single-node cluster", a.nodeID)

	// This is a recovery mechanism for orphaned nodes
	// Only attempt if we're truly isolated (no other nodes in cluster config)
	return a.store.ForceBootstrap(a.nodeID)
}
