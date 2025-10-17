package internal

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"scheduled-db/internal/api"
	"scheduled-db/internal/discovery"
	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"
)

type App struct {
	store            *store.Store
	slotQueue        *slots.SlotQueue
	worker           *slots.Worker
	httpServer       *http.Server
	discoveryManager *discovery.DiscoveryManager
	nodeID           string
	useDiscovery     bool
}

type Config struct {
	DataDir         string
	RaftBind        string
	HTTPBind        string
	NodeID          string
	Peers           []string
	SlotGap         time.Duration
	DiscoveryConfig discovery.DiscoveryConfig
}

func NewApp(config *Config) (*App, error) {
	// Create store with Raft (start with configured peers, discovery will handle dynamic joining)
	jobStore, err := store.NewStore(config.DataDir, config.RaftBind, config.NodeID, config.Peers)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %v", err)
	}

	// Wait for leader election (shorter timeout for nodes with peers)
	timeout := 30 * time.Second
	if len(config.Peers) > 0 {
		timeout = 5 * time.Second // Shorter timeout for joining nodes
	}

	if err := jobStore.WaitForLeader(timeout); err != nil {
		if len(config.Peers) > 0 {
			log.Printf("No leader found yet, will attempt join after discovery starts")
		} else {
			return nil, fmt.Errorf("failed to wait for leader: %v", err)
		}
	}

	// Create discovery manager only if discovery is enabled
	var discoveryManager *discovery.DiscoveryManager
	useDiscovery := config.DiscoveryConfig.Strategy != ""
	if useDiscovery {
		discoveryManager, err = discovery.NewDiscoveryManager(config.DiscoveryConfig, jobStore)
		if err != nil {
			return nil, fmt.Errorf("failed to create discovery manager: %v", err)
		}
	}

	// Create slot queue
	slotQueue := slots.NewSlotQueue(config.SlotGap)

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

	app := &App{
		store:            jobStore,
		slotQueue:        slotQueue,
		worker:           worker,
		httpServer:       httpServer,
		discoveryManager: discoveryManager,
		nodeID:           config.NodeID,
		useDiscovery:     useDiscovery,
	}

	// Setup event handler for job changes
	jobStore.SetEventHandler(func(event string, job *store.Job) {
		if jobStore.IsLeader() {
			switch event {
			case "created":
				if job != nil {
					slotQueue.AddJob(job)
					log.Printf("Added job %s to slot queue", job.ID)
				}
			case "deleted":
				if job != nil {
					slotQueue.RemoveJob(job.ID)
					log.Printf("Removed job %s from slot queue", job.ID)
				}
			}
		}
	})

	// Setup leadership change handler for all nodes (needed for high availability)
	go app.monitorLeadership()

	return app, nil
}

func (a *App) Start() error {
	log.Printf("Starting application on node %s", a.nodeID)

	// Start discovery manager only if enabled
	if a.useDiscovery {
		if err := a.discoveryManager.Start(); err != nil {
			return fmt.Errorf("failed to start discovery manager: %v", err)
		}

		// If we don't have a leader yet and have peers, try to trigger join via discovery
		if !a.store.IsLeader() && a.store.GetLeader() == "" && len(a.store.GetPeers()) > 0 {
			log.Printf("No leader found, discovery will help coordinate cluster join")
		}
	}

	// If this node becomes leader, load jobs and start worker
	if a.store.IsLeader() {
		a.becomeLeader()
	}

	// Start HTTP server
	go func() {
		log.Printf("Starting HTTP server on %s", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

func (a *App) Stop() error {
	log.Println("Stopping application...")

	// Use shorter timeouts and force shutdown if needed
	done := make(chan bool, 1)

	go func() {
		defer func() { done <- true }()

		// Stop discovery manager first (with short timeout) - only if enabled
		if a.useDiscovery {
			log.Println("Stopping discovery manager...")
			discoveryDone := make(chan error, 1)
			go func() {
				discoveryDone <- a.discoveryManager.Stop()
			}()

			select {
			case err := <-discoveryDone:
				if err != nil {
					log.Printf("Error stopping discovery manager: %v", err)
				}
			case <-time.After(10 * time.Second):
				log.Printf("Discovery manager shutdown timeout, continuing...")
			}
		}

		// Stop worker
		log.Println("Stopping worker...")
		a.worker.Stop()

		// Force close HTTP server (don't wait for graceful)
		log.Println("Force closing HTTP server...")
		if err := a.httpServer.Close(); err != nil {
			log.Printf("Error force closing HTTP server: %v", err)
		}

		// Close Raft store with timeout
		log.Println("Stopping Raft store...")
		storeDone := make(chan error, 1)
		go func() {
			storeDone <- a.store.Close()
		}()

		select {
		case err := <-storeDone:
			if err != nil {
				log.Printf("Error closing store: %v", err)
			}
		case <-time.After(10 * time.Second):
			log.Printf("Raft store shutdown timeout, forcing exit...")
		}
	}()

	// Force exit after 10 seconds total
	select {
	case <-done:
		log.Println("All components stopped successfully")
	case <-time.After(30 * time.Second):
		log.Println("Shutdown timeout reached, forcing exit")
	}

	return nil
}

func (a *App) monitorLeadership() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	wasLeader := a.store.IsLeader()
	log.Printf("[LEADERSHIP DEBUG] Starting leadership monitoring, initial state: isLeader=%v", wasLeader)
	noLeaderCount := 0

	for {
		select {
		case <-ticker.C:
			isLeader := a.store.IsLeader()
			currentLeader := a.store.GetLeader()

			if !wasLeader && isLeader {
				// Became leader
				log.Printf("[LEADERSHIP DEBUG] Node %s became leader", a.nodeID)
				a.becomeLeader()
				noLeaderCount = 0
			} else if wasLeader && !isLeader {
				// Lost leadership
				log.Printf("[LEADERSHIP DEBUG] Node %s lost leadership, current leader: %s", a.nodeID, currentLeader)
				a.loseLeadership()
			}

			// Check for orphaned cluster situation
			if currentLeader == "" && !isLeader {
				noLeaderCount++
				if noLeaderCount > 60 { // Wait 60 seconds without leader (increased from 10)
					servers, err := a.store.GetClusterConfiguration()
					if err == nil && len(servers) == 0 {
						// Empty cluster - attempt auto-bootstrap
						log.Printf("[LEADERSHIP DEBUG] Node %s: No leader for 60 seconds and empty cluster, attempting auto-bootstrap", a.nodeID)
						if err := a.attemptAutoBootstrap(); err != nil {
							log.Printf("[LEADERSHIP DEBUG] Auto-bootstrap failed: %v", err)
						} else {
							log.Printf("[LEADERSHIP DEBUG] Auto-bootstrap successful")
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
				log.Printf("[LEADERSHIP DEBUG] Node %s status: isLeader=%v, currentLeader=%s, raftState=%s, cluster=%s",
					a.nodeID, isLeader, currentLeader, raftState, clusterInfo)
			}

			wasLeader = isLeader
		}
	}
}

func (a *App) becomeLeader() {
	log.Printf("[LEADERSHIP DEBUG] Node %s becoming leader - loading jobs and starting worker", a.nodeID)

	// Load all jobs from store into slot queue
	jobs := a.store.GetAllJobs()
	a.slotQueue.LoadJobs(jobs)

	// Start worker
	a.worker.Start()

	log.Printf("[LEADERSHIP DEBUG] Node %s is now leader: loaded %d jobs into slot queue, worker started", a.nodeID, len(jobs))
}

func (a *App) loseLeadership() {
	log.Printf("[LEADERSHIP DEBUG] Node %s losing leadership - stopping worker", a.nodeID)
	a.worker.Stop()
	log.Printf("[LEADERSHIP DEBUG] Node %s worker stopped due to leadership loss", a.nodeID)
}

func (a *App) attemptAutoBootstrap() error {
	log.Printf("[LEADERSHIP DEBUG] Node %s attempting auto-bootstrap as single-node cluster", a.nodeID)

	// This is a recovery mechanism for orphaned nodes
	// Only attempt if we're truly isolated (no other nodes in cluster config)
	return a.store.ForceBootstrap(a.nodeID)
}
