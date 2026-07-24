package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"scheduled-db/internal"
	"scheduled-db/internal/logger"
	"scheduled-db/internal/slots"

	"github.com/manudiv16/pkgcluster"
)

func main() {
	var (
		boostrapexpect            = flag.Int("bootstrap-expect", getEnvIntOrDefault("BOOTSTRAP_EXPECT", 0), "The minimum number of nodes that the cluster will have")
		dataDir                   = flag.String("data-dir", getEnvOrDefault("DATA_DIR", "./data"), "Data directory for Raft storage")
		raftPort                  = flag.String("raft-port", getEnvOrDefault("RAFT_PORT", "7000"), "Port for Raft communication")
		httpPort                  = flag.String("http-port", getEnvOrDefault("HTTP_PORT", "8080"), "Port for HTTP API")
		nodeID                    = flag.String("node-id", getEnvOrDefault("NODE_ID", "node-1"), "Unique node identifier")
		peers                     = flag.String("peers", getEnvOrDefault("PEERS", ""), "Comma-separated list of peer addresses for joining cluster")
		slotGap                   = flag.Duration("slot-gap", getEnvDurationOrDefault("SLOT_GAP", 10*time.Second), "Time gap for slot intervals")
		discoveryStrategy         = flag.String("discovery-strategy", getEnvOrDefault("DISCOVERY_STRATEGY", ""), "Discovery strategy: static, kubernetes, dns, gossip")
		kubernetesNamespace       = flag.String("kubernetes-namespace", getEnvOrDefault("KUBERNETES_NAMESPACE", ""), "kubernetes Namespace")
		kubernetesServiceName     = flag.String("kubernetes-service-name", getEnvOrDefault("KUBERNETES_SERVICE_NAME", ""), "kubernetes service name")
		raftHost                  = flag.String("raft-host", getEnvOrDefault("RAFT_HOST", "localhost"), "Host for Raft communication")
		raftAdvertiseHost         = flag.String("raft-advertise-host", getEnvOrDefault("RAFT_ADVERTISE_HOST", ""), "Host to advertise for Raft communication (empty means use raft-host)")
		httpHost                  = flag.String("http-host", getEnvOrDefault("HTTP_HOST", ""), "Host for HTTP API (empty means all interfaces)")
		executionTimeout          = flag.Duration("execution-timeout", getEnvDurationOrDefault("JOB_EXECUTION_TIMEOUT", 5*time.Minute), "Job execution timeout")
		inProgressTimeout         = flag.Duration("inprogress-timeout", getEnvDurationOrDefault("JOB_INPROGRESS_TIMEOUT", 5*time.Minute), "In-progress job timeout")
		maxExecutionAttempts      = flag.Int("max-attempts", getEnvIntOrDefault("MAX_EXECUTION_ATTEMPTS", 3), "Maximum execution attempts per job")
		historyRetention          = flag.Duration("history-retention", getEnvDurationOrDefault("EXECUTION_HISTORY_RETENTION", 30*24*time.Hour), "Execution history retention period")
		healthFailureThreshold    = flag.Float64("health-failure-threshold", getEnvFloatOrDefault("HEALTH_FAILURE_THRESHOLD", 0.1), "Health check failure threshold (ratio of failed jobs)")
		queueMemoryLimit          = flag.String("queue-memory-limit", getEnvOrDefault("QUEUE_MEMORY_LIMIT", ""), "Queue memory limit (e.g., 2GB, 500MB) - empty means auto-detect")
		queueMemoryPercent        = flag.Float64("queue-memory-percent", getEnvFloatOrDefault("QUEUE_MEMORY_PERCENT", 50.0), "Queue memory as percentage of system memory (default 50%)")
		queueJobLimit             = flag.Int64("queue-job-limit", getEnvInt64OrDefault("QUEUE_JOB_LIMIT", 100000), "Maximum number of jobs in queue (default 100,000)")
		enableColdSpilling        = flag.Bool("enable-cold-spilling", getEnvBoolOrDefault("ENABLE_COLD_SPILLING", false), "Enable cold spilling for slots (archive old slots to disk)")
		coldSpillingHotWindow     = flag.Duration("cold-spilling-hot-window", getEnvDurationOrDefault("COLD_SPILLING_HOT_WINDOW", 48*time.Hour), "Time window for hot slots in memory (default 48h)")
		coldSpillingCheckInterval = flag.Duration("cold-spilling-check-interval", getEnvDurationOrDefault("COLD_SPILLING_CHECK_INTERVAL", 5*time.Minute), "Interval for eviction checks (default 5m)")
		wheelLevel0Granularity    = flag.Duration("htw-level0-granularity", getEnvDurationOrDefault("HTW_LEVEL0_GRANULARITY", 0), "Timing wheel level 0 granularity (default: slot-gap * 1)")
		wheelLevel0Buckets        = flag.Int("htw-level0-buckets", getEnvIntOrDefault("HTW_LEVEL0_BUCKETS", 360), "Timing wheel level 0 buckets (default: 360)")
		wheelLevel1Granularity    = flag.Duration("htw-level1-granularity", getEnvDurationOrDefault("HTW_LEVEL1_GRANULARITY", 0), "Timing wheel level 1 granularity (default: slot-gap * 360)")
		wheelLevel1Buckets        = flag.Int("htw-level1-buckets", getEnvIntOrDefault("HTW_LEVEL1_BUCKETS", 24), "Timing wheel level 1 buckets (default: 24)")
		wheelLevel2Granularity    = flag.Duration("htw-level2-granularity", getEnvDurationOrDefault("HTW_LEVEL2_GRANULARITY", 0), "Timing wheel level 2 granularity (default: slot-gap * 360 * 24)")
		wheelLevel2Buckets        = flag.Int("htw-level2-buckets", getEnvIntOrDefault("HTW_LEVEL2_BUCKETS", 365), "Timing wheel level 2 buckets (default: 365)")

		// Security flags
		httpReadTimeout       = flag.Duration("http-read-timeout", getEnvDurationOrDefault("HTTP_READ_TIMEOUT", 30*time.Second), "HTTP read timeout")
		httpReadHeaderTimeout = flag.Duration("http-read-header-timeout", getEnvDurationOrDefault("HTTP_READ_HEADER_TIMEOUT", 10*time.Second), "HTTP read header timeout")
		httpWriteTimeout      = flag.Duration("http-write-timeout", getEnvDurationOrDefault("HTTP_WRITE_TIMEOUT", 30*time.Second), "HTTP write timeout")
		httpIdleTimeout       = flag.Duration("http-idle-timeout", getEnvDurationOrDefault("HTTP_IDLE_TIMEOUT", 60*time.Second), "HTTP idle timeout")
		maxRequestBodySize    = flag.Int64("max-request-body-size", getEnvInt64OrDefault("MAX_REQUEST_BODY_SIZE", 10*1024*1024), "Maximum request body size in bytes (default 10MB)")
		authToken             = flag.String("auth-token", getEnvOrDefault("AUTH_TOKEN", ""), "Shared secret for cluster join authentication (empty = disabled)")
	)
	flag.Parse()

	// Parse peers list
	var peerList []string
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
		for i, peer := range peerList {
			peerList[i] = strings.TrimSpace(peer)
		}
	}

	// Determine discovery strategy.
	strategy := *discoveryStrategy
	if strategy == "" {
		if envStrategy := os.Getenv("DISCOVERY_STRATEGY"); envStrategy != "" {
			strategy = envStrategy
		} else {
			strategy = "none"
		}
	}

	// Build topology list from the selected strategy.
	// Callbacks (Connect/Disconnect/ListNodes) are nil here and will be
	// wired to the Raft store by internal.NewApp.
	var topologies []pkgcluster.Topology
	if strategy != "none" {
		topo, err := buildTopology(strategy, *nodeID, peerList, *kubernetesNamespace, *kubernetesServiceName)
		if err != nil {
			logger.Error("failed to build discovery topology: %v", err)
			os.Exit(1)
		}
		topologies = append(topologies, topo)
	}

	// Build bind addresses with environment variables
	raftBind := fmt.Sprintf("%s:%s", *raftHost, *raftPort)

	advertiseHost := *raftAdvertiseHost
	if advertiseHost == "" {
		advertiseHost = *raftHost
	}
	raftAdvertise := fmt.Sprintf("%s:%s", advertiseHost, *raftPort)

	httpBind := fmt.Sprintf("%s:%s", *httpHost, *httpPort)

	// Detect or configure memory limit
	memoryLimit := DetectMemoryLimit(*queueMemoryLimit, *queueMemoryPercent)

	// Validate job limit
	jobLimit := *queueJobLimit
	if jobLimit <= 0 {
		logger.Warn("invalid QUEUE_JOB_LIMIT: %d, using default 100,000", jobLimit)
		jobLimit = 100000
	}
	logger.Info("using job count limit: %d jobs", jobLimit)

	var wheelConfigs []slots.WheelLevelConfig
	l0g := *wheelLevel0Granularity
	if l0g == 0 {
		l0g = *slotGap
	}
	l1g := *wheelLevel1Granularity
	if l1g == 0 {
		l1g = *slotGap * 360
	}
	l2g := *wheelLevel2Granularity
	if l2g == 0 {
		l2g = *slotGap * 360 * 24
	}
	wheelConfigs = []slots.WheelLevelConfig{
		{Granularity: l0g, Buckets: *wheelLevel0Buckets},
		{Granularity: l1g, Buckets: *wheelLevel1Buckets},
		{Granularity: l2g, Buckets: *wheelLevel2Buckets},
	}

	config := &internal.Config{
		DataDir:                   *dataDir,
		RaftBind:                  raftBind,
		RaftAdvertise:             raftAdvertise,
		HTTPBind:                  httpBind,
		NodeID:                    *nodeID,
		Peers:                     peerList,
		SlotGap:                   *slotGap,
		Topologies:                topologies,
		ExecutionTimeout:          *executionTimeout,
		InProgressTimeout:         *inProgressTimeout,
		MaxExecutionAttempts:      *maxExecutionAttempts,
		HistoryRetention:          *historyRetention,
		HealthFailureThreshold:    *healthFailureThreshold,
		QueueMemoryLimit:          memoryLimit,
		QueueJobLimit:             jobLimit,
		EnableColdSpilling:        *enableColdSpilling,
		ColdSpillingHotWindow:     *coldSpillingHotWindow,
		ColdSpillingCheckInterval: *coldSpillingCheckInterval,
		TimingWheelConfigs:        wheelConfigs,
		BootstrapExpect:           *boostrapexpect,

		// Security configuration
		HTTPReadTimeout:       *httpReadTimeout,
		HTTPReadHeaderTimeout: *httpReadHeaderTimeout,
		HTTPWriteTimeout:      *httpWriteTimeout,
		HTTPIdleTimeout:       *httpIdleTimeout,
		MaxRequestBodySize:    *maxRequestBodySize,
		AuthToken:             *authToken,
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
	logger.Info("queue memory limit: %d bytes (%.2f GB)", memoryLimit, float64(memoryLimit)/(1024*1024*1024))
	logger.Info("queue job limit: %d jobs", jobLimit)
	if *enableColdSpilling {
		logger.Info("cold spilling: enabled, hot_window=%s, check_interval=%s", *coldSpillingHotWindow, *coldSpillingCheckInterval)
	} else {
		logger.Info("cold spilling: disabled")
	}
	logger.Info("timing wheel: L0=%s×%d, L1=%s×%d, L2=%s×%d",
		l0g, *wheelLevel0Buckets, l1g, *wheelLevel1Buckets, l2g, *wheelLevel2Buckets)
	if *boostrapexpect > 0 {
		logger.Info("bootstrap-expect: %d, will wait for peers before bootstrapping cluster", *boostrapexpect)
	} else if len(peerList) > 0 {
		logger.Info("peers: %v", peerList)
	} else {
		logger.Info("running in single-node (bootstrap) mode")
	}
	if len(topologies) > 0 {
		logger.Info("discovery: %s strategy configured for topology %q", strategy, topologies[0].Name)
	} else {
		logger.Info("discovery: disabled")
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

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
				os.Exit(130)
			}
		}
	}()

	<-shutdownDone
	logger.Info("application stopped successfully")
	os.Exit(0)
}

// buildTopology creates a pkgcluster.Topology from CLI flags and environment
// variables. It only sets the strategy type and config map; callbacks are
// wired later by internal.NewApp.
func buildTopology(strategy, nodeID string, peers []string, kubernetesNamespace, kubernetesServiceName string) (pkgcluster.Topology, error) {
	raftPortStr := getEnvOrDefault("RAFT_PORT", "7000")
	raftPort, _ := strconv.Atoi(raftPortStr)

	var topo pkgcluster.Topology

	switch strategy {
	case "static":
		topo = pkgcluster.Topology{
			Name:     "static",
			Strategy: pkgcluster.StrategyStatic,
			Config: map[string]interface{}{
				"addresses": peers,
			},
		}

	case "kubernetes":
		topo = pkgcluster.Topology{
			Name:     "kubernetes",
			Strategy: pkgcluster.StrategyKubernetes,
			Config: map[string]interface{}{
				"namespace":     kubernetesNamespace,
				"selector":      fmt.Sprintf("app=%s", kubernetesServiceName),
				"node_basename": nodeID,
				"service_name":  kubernetesServiceName,
				"port":          raftPort,
			},
		}

	case "dns":
		topo = pkgcluster.Topology{
			Name:     "dns",
			Strategy: pkgcluster.StrategyDNS,
			Config: map[string]interface{}{
				"query":         kubernetesServiceName,
				"node_basename": nodeID,
				"port":          raftPort,
			},
		}

	case "kubernetes_dns":
		topo = pkgcluster.Topology{
			Name:     "kubernetes_dns",
			Strategy: pkgcluster.StrategyKubernetesDNS,
			Config: map[string]interface{}{
				"service":          kubernetesServiceName,
				"application_name": nodeID,
				"port":             raftPort,
			},
		}

	case "kubernetes_dns_srv", "dns_srv":
		topo = pkgcluster.Topology{
			Name:     "kubernetes_dns_srv",
			Strategy: pkgcluster.StrategyKubernetesDNSSRV,
			Config: map[string]interface{}{
				"service":          kubernetesServiceName,
				"application_name": nodeID,
				"namespace":        kubernetesNamespace,
				"port":             raftPort,
			},
		}

	case "gossip":
		topo = pkgcluster.Topology{
			Name:     "gossip",
			Strategy: pkgcluster.StrategyGossip,
			Config: map[string]interface{}{
				"port":      45892,
				"node_addr": fmt.Sprintf("%s:%s", nodeID, raftPortStr),
			},
		}

	default:
		return topo, fmt.Errorf("unsupported discovery strategy: %s", strategy)
	}

	return topo, nil
}

// --- Helper functions (env vars, parsing) ---

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

func getEnvInt64OrDefault(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
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

// ParseMemoryLimit parses memory limit strings like "1GB", "500MB", "1073741824"
func ParseMemoryLimit(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty memory limit value")
	}

	multiplier := int64(1)
	if strings.HasSuffix(strings.ToUpper(value), "GB") {
		multiplier = 1024 * 1024 * 1024
		value = strings.TrimSuffix(strings.TrimSuffix(value, "GB"), "gb")
	} else if strings.HasSuffix(strings.ToUpper(value), "MB") {
		multiplier = 1024 * 1024
		value = strings.TrimSuffix(strings.TrimSuffix(value, "MB"), "mb")
	} else if strings.HasSuffix(strings.ToUpper(value), "KB") {
		multiplier = 1024
		value = strings.TrimSuffix(strings.TrimSuffix(value, "KB"), "kb")
	}

	value = strings.TrimSpace(value)
	num, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit format: %v", err)
	}

	if num <= 0 {
		return 0, fmt.Errorf("memory limit must be positive")
	}

	return num * multiplier, nil
}

// DetectMemoryLimit determines memory limit from config or system memory
func DetectMemoryLimit(explicitLimit string, memoryPercent float64) int64 {
	if explicitLimit != "" {
		limit, err := ParseMemoryLimit(explicitLimit)
		if err != nil {
			logger.Error("invalid QUEUE_MEMORY_LIMIT: %v, falling back to auto-detection", err)
		} else {
			logger.Info("using configured memory limit: %d bytes (%.2f GB)", limit, float64(limit)/(1024*1024*1024))
			return limit
		}
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	systemMemory := int64(m.Sys)

	percent := memoryPercent
	if percent <= 0 || percent > 100 {
		percent = 50.0
	}

	limit := int64(float64(systemMemory) * (percent / 100.0))

	logger.Info("detected memory limit: %d bytes (%.2f GB) - %.1f%% of %d bytes (%.2f GB) system memory",
		limit, float64(limit)/(1024*1024*1024), percent, systemMemory, float64(systemMemory)/(1024*1024*1024))

	return limit
}
