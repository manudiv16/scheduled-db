package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"scheduled-db/internal"
	"scheduled-db/internal/discovery"
)

func main() {
	var (
		dataDir           = flag.String("data-dir", getEnvOrDefault("DATA_DIR", "./data"), "Data directory for Raft storage")
		raftPort          = flag.String("raft-port", getEnvOrDefault("RAFT_PORT", "7000"), "Port for Raft communication")
		httpPort          = flag.String("http-port", getEnvOrDefault("HTTP_PORT", "8080"), "Port for HTTP API")
		nodeID            = flag.String("node-id", getEnvOrDefault("NODE_ID", "node-1"), "Unique node identifier")
		peers             = flag.String("peers", getEnvOrDefault("PEERS", ""), "Comma-separated list of peer addresses for joining cluster")
		slotGap           = flag.Duration("slot-gap", getEnvDurationOrDefault("SLOT_GAP", 10*time.Second), "Time gap for slot intervals")
		discoveryStrategy = flag.String("discovery-strategy", getEnvOrDefault("DISCOVERY_STRATEGY", ""), "Discovery strategy: static, kubernetes, dns, gossip")
		raftHost          = flag.String("raft-host", getEnvOrDefault("RAFT_HOST", "localhost"), "Host for Raft communication")
		httpHost          = flag.String("http-host", getEnvOrDefault("HTTP_HOST", ""), "Host for HTTP API (empty means all interfaces)")
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
	httpBind := fmt.Sprintf("%s:%s", *httpHost, *httpPort)

	// Create application configuration
	config := &internal.Config{
		DataDir:         *dataDir,
		RaftBind:        raftBind,
		HTTPBind:        httpBind,
		NodeID:          *nodeID,
		Peers:           peerList,
		SlotGap:         *slotGap,
		DiscoveryConfig: discoveryConfig,
	}

	// Create and start application
	app, err := internal.NewApp(config)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	log.Printf("Application started successfully")
	log.Printf("Node ID: %s", *nodeID)
	log.Printf("Raft port: %s", *raftPort)
	log.Printf("HTTP port: %s", *httpPort)
	if len(peerList) > 0 {
		log.Printf("Peers: %v", peerList)
	} else {
		log.Printf("Running in single-node (bootstrap) mode")
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
				log.Printf("Received signal: %v, shutting down gracefully... (Ctrl+C again to force quit)", sig)
				go func() {
					if err := app.Stop(); err != nil {
						log.Printf("Error during shutdown: %v", err)
						os.Exit(1)
					}
					shutdownDone <- true
				}()
			} else if shutdownCount >= 2 {
				log.Printf("Force quit requested, exiting immediately...")
				os.Exit(130) // Standard exit code for Ctrl+C
			}
		}
	}()

	// Wait for graceful shutdown (no automatic timeout)
	<-shutdownDone
	log.Println("Application stopped successfully")
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
