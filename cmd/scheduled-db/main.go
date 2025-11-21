package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"scheduled-db/internal"
	"scheduled-db/internal/discovery"
	"scheduled-db/internal/logger"
)

func main() {
	var (
		dataDir                = flag.String("data-dir", getEnvOrDefault("DATA_DIR", "./data"), "Data directory for Raft storage")
		raftPort               = flag.String("raft-port", getEnvOrDefault("RAFT_PORT", "7000"), "Port for Raft communication")
		httpPort               = flag.String("http-port", getEnvOrDefault("HTTP_PORT", "8080"), "Port for HTTP API")
		nodeID                 = flag.String("node-id", getEnvOrDefault("NODE_ID", "node-1"), "Unique node identifier")
		peers                  = flag.String("peers", getEnvOrDefault("PEERS", ""), "Comma-separated list of peer addresses for joining cluster")
		slotGap                = flag.Duration("slot-gap", getEnvDurationOrDefault("SLOT_GAP", 10*time.Second), "Time gap for slot intervals")
		discoveryStrategy      = flag.String("discovery-strategy", getEnvOrDefault("DISCOVERY_STRATEGY", ""), "Discovery strategy: static, kubernetes, dns, gossip")
		raftHost               = flag.String("raft-host", getEnvOrDefault("RAFT_HOST", "localhost"), "Host for Raft communication")
		raftAdvertiseHost      = flag.String("raft-advertise-host", getEnvOrDefault("RAFT_ADVERTISE_HOST", ""), "Host to advertise for Raft communication (empty means use raft-host)")
		httpHost               = flag.String("http-host", getEnvOrDefault("HTTP_HOST", ""), "Host for HTTP API (empty means all interfaces)")
		executionTimeout       = flag.Duration("execution-timeout", getEnvDurationOrDefault("JOB_EXECUTION_TIMEOUT", 5*time.Minute), "Job execution timeout")
		inProgressTimeout      = flag.Duration("inprogress-timeout", getEnvDurationOrDefault("JOB_INPROGRESS_TIMEOUT", 5*time.Minute), "In-progress job timeout")
		maxExecutionAttempts   = flag.Int("max-attempts", getEnvIntOrDefault("MAX_EXECUTION_ATTEMPTS", 3), "Maximum execution attempts per job")
		historyRetention       = flag.Duration("history-retention", getEnvDurationOrDefault("EXECUTION_HISTORY_RETENTION", 30*24*time.Hour), "Execution history retention period")
		healthFailureThreshold = flag.Float64("health-failure-threshold", getEnvFloatOrDefault("HEALTH_FAILURE_THRESHOLD", 0.1), "Health check failure threshold (ratio of failed jobs)")
	)
	flag.Parse()

	// Determine discovery strategy - make it truly optional
	strategy := *discoveryStrategy
	if strategy == "" {
		if envStrategy := os.Getenv("DISCOVERY_STRATEGY"); envStrategy != "" {
			strategy = envStrategy
		} else {
			// No discovery strategy = traditional Raft only
			strategy = "none"
		}
	}

	// Parse peers for static strategy
	var peerList []string
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
		for i, peer := range peerList {
			peerList[i] = strings.TrimSpace(peer)
		}
	}

	// Create discovery configuration (only if needed)
	var discoveryConfig discovery.DiscoveryConfig
	if strategy != "none" {
		discoveryConfig = createDiscoveryConfig(strategy, *nodeID, peerList)
	}

	// Build bind addresses with environment variables
	raftBind := fmt.Sprintf("%s:%s", *raftHost, *raftPort)

	// Determine advertise address - use advertise host if provided, otherwise use raft host
	advertiseHost := *raftAdvertiseHost
	if advertiseHost == "" {
		advertiseHost = *raftHost
	}
	raftAdvertise := fmt.Sprintf("%s:%s", advertiseHost, *raftPort)

	httpBind := fmt.Sprintf("%s:%s", *httpHost, *httpPort)

	// Create application configuration
	config := &internal.Config{
		DataDir:                *dataDir,
		RaftBind:               raftBind,
		RaftAdvertise:          raftAdvertise,
		HTTPBind:               httpBind,
		NodeID:                 *nodeID,
		Peers:                  peerList,
		SlotGap:                *slotGap,
		DiscoveryConfig:        discoveryConfig,
		ExecutionTimeout:       *executionTimeout,
		InProgressTimeout:      *inProgressTimeout,
		MaxExecutionAttempts:   *maxExecutionAttempts,
		HistoryRetention:       *historyRetention,
		HealthFailureThreshold: *healthFailureThreshold,
	}

	// Create and start application
	app, err := internal.NewApp(config)
	if err != nil {
		logger.Error("failed to create application: %v", err)
		os.Exit(1)
	}

	if err := app.Start(); err != nil {
		logger.Error("failed to start application: %v", err)
		os.Exit(1)
	}

	logger.Info("application started successfully")
	logger.Info("node ID: %s", *nodeID)
	logger.Info("raft bind: %s", raftBind)
	logger.Info("raft advertise: %s", raftAdvertise)
	logger.Info("HTTP bind: %s", httpBind)
	if len(peerList) > 0 {
		logger.Info("peers: %v", peerList)
	} else {
		logger.Info("running in single-node (bootstrap) mode")
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Handle shutdown with force-quit on multiple Ctrl+C
	var shutdownCount int
	shutdownDone := make(chan bool, 1)

	go func() {
		for {
			sig := <-sigCh
			shutdownCount++

			if shutdownCount == 1 {
				logger.Info("received signal: %v, shutting down gracefully... (Ctrl+C again to force quit)", sig)
				go func() {
					if err := app.Stop(); err != nil {
						logger.Error("error during shutdown: %v", err)
						os.Exit(1)
					}
					shutdownDone <- true
				}()
			} else if shutdownCount >= 2 {
				logger.Info("force quit requested, exiting immediately...")
				os.Exit(130) // Standard exit code for Ctrl+C
			}
		}
	}()

	// Wait for graceful shutdown (no automatic timeout)
	<-shutdownDone
	logger.Info("application stopped successfully")
	os.Exit(0)
}

func createDiscoveryConfig(strategyStr, nodeID string, staticPeers []string) discovery.DiscoveryConfig {
	var strategy discovery.StrategyType
	switch strategyStr {
	case "kubernetes":
		strategy = discovery.StrategyKubernetes
	case "dns":
		strategy = discovery.StrategyDNS
	case "gossip":
		strategy = discovery.StrategyGossip
	case "consul":
		strategy = discovery.StrategyConsul
	default:
		strategy = discovery.StrategyStatic
	}

	config := discovery.DiscoveryConfig{
		Config: discovery.Config{
			NodeID:      nodeID,
			ServiceName: "scheduled-db",
			Namespace:   getEnvOrDefault("NAMESPACE", "default"),
			Interval:    30 * time.Second,
			Meta:        make(map[string]string),
		},
		Strategy:       strategy,
		AutoJoin:       getEnvBoolOrDefault("AUTO_JOIN", true),
		UpdateInterval: 30 * time.Second,
	}

	// Add static peers if provided
	if len(staticPeers) > 0 {
		config.Config.Meta["peers"] = strings.Join(staticPeers, ",")
	}

	// Add strategy-specific configuration
	switch strategy {
	case discovery.StrategyKubernetes:
		config.KubernetesConfig = &discovery.KubernetesConfig{
			InCluster:    getEnvBoolOrDefault("KUBERNETES_IN_CLUSTER", true),
			PodNamespace: getEnvOrDefault("POD_NAMESPACE", "default"),
			ServiceName:  "scheduled-db",
		}
	case discovery.StrategyGossip:
		config.GossipConfig = &discovery.GossipConfig{
			BindPort: getEnvIntOrDefault("GOSSIP_PORT", 7946),
		}
		if seeds := os.Getenv("GOSSIP_SEEDS"); seeds != "" {
			config.Config.Meta["seeds"] = seeds
		}
	case discovery.StrategyDNS:
		config.DNSConfig = &discovery.DNSConfig{
			Domain:       getEnvOrDefault("DNS_DOMAIN", "cluster.local"),
			PollInterval: 30 * time.Second,
		}
	}

	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}
