package store

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// getApplyTimeout returns the Raft apply timeout from environment or default
func getApplyTimeout() time.Duration {
	if timeoutStr := os.Getenv("RAFT_APPLY_TIMEOUT"); timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil {
			return t
		}
	}
	return 5 * time.Second // Default
}

type JobEventHandler func(event string, job *Job)

type Store struct {
	raft          *raft.Raft
	fsm           *FSM
	transport     raft.Transport
	eventHandler  JobEventHandler
	nodeID        string
	raftBind      string
	raftAdvertise string
	httpBind      string
}

func NewStore(dataDir, raftBind, raftAdvertise, nodeID string, peers []string) (*Store, error) {
	// Use stable hostname for Raft advertise address if in Kubernetes
	if os.Getenv("POD_NAME") != "" && os.Getenv("DISCOVERY_STRATEGY") == "kubernetes" {
		serviceName := os.Getenv("KUBERNETES_SERVICE_NAME")
		namespace := os.Getenv("POD_NAMESPACE")
		if serviceName == "" {
			serviceName = "scheduled-db"
		}
		if namespace == "" {
			namespace = "default"
		}
		podName := os.Getenv("POD_NAME")
		
		// Use stable hostname instead of IP
		_, port, err := net.SplitHostPort(raftAdvertise)
		if err == nil {
			stableHostname := fmt.Sprintf("%s.%s.%s.svc.cluster.local", podName, serviceName, namespace)
			raftAdvertise = fmt.Sprintf("%s:%s", stableHostname, port)
			log.Printf("Using stable hostname for Raft: %s", raftAdvertise)
		}
	}
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	// Configure more aggressive timeouts for faster leader detection
	config.HeartbeatTimeout = 1000 * time.Millisecond
	config.ElectionTimeout = 1000 * time.Millisecond
	config.CommitTimeout = 50 * time.Millisecond
	config.LeaderLeaseTimeout = 500 * time.Millisecond

	fsm := NewFSM()

	// Create data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	// Setup Raft transport
	advertiseAddr, err := net.ResolveTCPAddr("tcp", raftAdvertise)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve raft advertise address: %v", err)
	}

	// Get timeout from environment or use default
	timeout := 10 * time.Second
	if timeoutStr := os.Getenv("RAFT_TRANSPORT_TIMEOUT"); timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = t
		}
	}
	transport, err := raft.NewTCPTransport(raftBind, advertiseAddr, 3, timeout, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(dataDir, 2, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %v", err)
	}

	// Create the log store and stable store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "logs.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %v", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "stable.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create stable store: %v", err)
	}

	// Instantiate the Raft systems
	ra, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshots, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %v", err)
	}

	store := &Store{
		raft:          ra,
		fsm:           fsm,
		transport:     transport,
		nodeID:        nodeID,
		raftBind:      raftBind,
		raftAdvertise: raftAdvertise,
	}

	// Only bootstrap if no peers provided (single node/bootstrap mode)
	if len(peers) == 0 {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		bootstrap := ra.BootstrapCluster(configuration)
		if err := bootstrap.Error(); err != nil {
			log.Printf("Failed to bootstrap cluster: %v", err)
		} else {
			log.Printf("Successfully bootstrapped single-node cluster with ID: %s, Address: %s", config.LocalID, transport.LocalAddr())
		}
	} else {
		// Don't bootstrap if we have peers - wait to be added by leader via join API
		log.Printf("Starting as follower node ID: %s, Address: %s, will join cluster via discovery and /join API with peers: %v",
			config.LocalID, transport.LocalAddr(), peers)
	}

	log.Printf("Raft store initialized - Node ID: %s, Bind: %s, Advertise: %s, Initial state: %s", nodeID, raftBind, raftAdvertise, ra.State())
	return store, nil
}

// SetHTTPBind sets the HTTP bind address for this store
func (s *Store) SetHTTPBind(httpBind string) {
	s.httpBind = httpBind
}

// GetNodeID returns the node ID
func (s *Store) GetNodeID() string {
	return s.nodeID
}

// GetRaftBind returns the Raft bind address
func (s *Store) GetRaftBind() string {
	return s.raftBind
}

// GetRaftAdvertise returns the Raft advertise address
func (s *Store) GetRaftAdvertise() string {
	return s.raftAdvertise
}

// GetHTTPBind returns the HTTP bind address
func (s *Store) GetHTTPBind() string {
	return s.httpBind
}

// SetEventHandler sets the callback for job events
func (s *Store) SetEventHandler(handler JobEventHandler) {
	s.eventHandler = handler
}

// CreateJob creates a new job
func (s *Store) CreateJob(job *Job) error {
	if !s.IsLeader() {
		return fmt.Errorf("not leader")
	}

	command := Command{
		Type: CommandCreateJob,
		Job:  job,
	}

	data, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}

	future := s.raft.Apply(data, getApplyTimeout())
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %v", err)
	}

	// Notify event handler if this is the leader
	if s.eventHandler != nil && s.IsLeader() {
		s.eventHandler("created", job)
	}

	return nil
}

// DeleteJob deletes a job
func (s *Store) DeleteJob(id string) error {
	if !s.IsLeader() {
		return fmt.Errorf("not leader")
	}

	command := Command{
		Type: CommandDeleteJob,
		ID:   id,
	}

	data, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}

	future := s.raft.Apply(data, getApplyTimeout())
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %v", err)
	}

	// Notify event handler if this is the leader
	if s.eventHandler != nil && s.IsLeader() {
		job, _ := s.GetJob(id)
		s.eventHandler("deleted", job)
	}

	return nil
}

// GetJob returns a job by ID
func (s *Store) GetJob(id string) (*Job, bool) {
	return s.fsm.GetJob(id)
}

// GetAllJobs returns all jobs
func (s *Store) GetAllJobs() map[string]*Job {
	return s.fsm.GetAllJobs()
}

// IsLeader returns true if this node is the leader
func (s *Store) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

// GetLeader returns the current leader address
func (s *Store) GetLeader() string {
	return string(s.raft.Leader())
}

// GetRaftState returns the current Raft state as a string
func (s *Store) GetRaftState() string {
	return s.raft.State().String()
}

// WaitForLeader waits until a leader is elected
func (s *Store) WaitForLeader(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.raft.Leader() != "" {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for leader")
}

// GetRaft returns the underlying Raft instance
func (s *Store) GetRaft() *raft.Raft {
	return s.raft
}

// AddPeer adds a new peer to the Raft cluster (only if leader)
func (s *Store) AddPeer(id, address string) error {
	if !s.IsLeader() {
		return fmt.Errorf("not leader, cannot add peer")
	}

	serverID := raft.ServerID(id)
	serverAddr := raft.ServerAddress(address)

	// Check if peer already exists
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to get configuration: %v", err)
	}

	config := future.Configuration()
	for _, server := range config.Servers {
		if server.ID == serverID || server.Address == serverAddr {
			log.Printf("Peer %s (%s) already exists in cluster", id, address)
			return nil // Already exists, not an error
		}
	}

	// Add as voter
	addFuture := s.raft.AddVoter(serverID, serverAddr, 0, 0)
	if err := addFuture.Error(); err != nil {
		return fmt.Errorf("failed to add peer: %v", err)
	}

	log.Printf("Successfully added peer %s (%s) to Raft cluster", id, address)
	return nil
}

// RemovePeer removes a peer from the Raft cluster (only if leader)
func (s *Store) RemovePeer(id string) error {
	if !s.IsLeader() {
		return fmt.Errorf("not leader, cannot remove peer")
	}

	serverID := raft.ServerID(id)
	future := s.raft.RemoveServer(serverID, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to remove peer: %v", err)
	}

	log.Printf("Successfully removed peer %s from Raft cluster", id)
	return nil
}

// GetClusterConfiguration returns current Raft cluster configuration
func (s *Store) GetClusterConfiguration() ([]raft.Server, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to get configuration: %v", err)
	}

	config := future.Configuration()
	return config.Servers, nil
}

// GetPeers returns the configured peers (for debugging)
func (s *Store) GetPeers() []string {
	// This is a simplified implementation - in a real scenario
	// you'd track the original peer list or get it from config
	servers, err := s.GetClusterConfiguration()
	if err != nil {
		return []string{}
	}

	var peers []string
	localAddr := s.raft.String()
	for _, server := range servers {
		addr := string(server.Address)
		if addr != localAddr {
			peers = append(peers, addr)
		}
	}
	return peers
}

// ForceBootstrap attempts to bootstrap this node as a single-node cluster
// This is a recovery mechanism for orphaned nodes
func (s *Store) ForceBootstrap(nodeID string) error {
	// Check if we're already in a cluster
	servers, err := s.GetClusterConfiguration()
	if err == nil && len(servers) > 0 {
		return fmt.Errorf("node is already part of a cluster with %d servers", len(servers))
	}

	// Check if we're already a leader
	if s.IsLeader() {
		return fmt.Errorf("node is already a leader")
	}

	// Create bootstrap configuration
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(nodeID),
				Address: s.transport.LocalAddr(),
			},
		},
	}

	// Attempt bootstrap
	bootstrap := s.raft.BootstrapCluster(configuration)
	if err := bootstrap.Error(); err != nil {
		return fmt.Errorf("failed to force bootstrap: %v", err)
	}

	log.Printf("Successfully force-bootstrapped node %s as single-node cluster", nodeID)
	return nil
}

// Close closes the store
func (s *Store) Close() error {
	return s.raft.Shutdown().Error()
}
