//go:build !wasm

package store

import (
	"time"

	"github.com/hashicorp/raft"
)

// JobStore defines read and write operations for jobs.
type JobStore interface {
	CreateJob(job *Job) error
	DeleteJob(id string) error
	GetJob(id string) (*Job, bool)
	GetAllJobs() map[string]*Job
	GetExecutionState(jobID string) (*JobExecutionState, bool)
	GetAllExecutionStates() map[string]*JobExecutionState
}

// SlotStore defines read and write operations for time slots.
type SlotStore interface {
	CreateSlot(slot *SlotData) error
	DeleteSlot(key int64) error
	GetSlot(key int64) (*SlotData, bool)
	GetAllSlots() map[int64]*SlotData
	ArchiveSlot(key int64) error
	UnarchiveSlot(key int64) error
	IsSlotCold(key int64) bool
	IsColdSpillingEnabled() bool
	GetHotSlotCount() int
	GetColdSlotCount() int
	GetColdSlotKeys() []int64
	GetColdSlotData(key int64) (*SlotData, error)
}

// ClusterStore defines Raft cluster management operations.
type ClusterStore interface {
	IsLeader() bool
	GetLeader() string
	GetRaftState() string
	WaitForLeader(timeout time.Duration) error
	AddPeer(id, address string) error
	RemovePeer(id string) error
	GetClusterConfiguration() ([]raft.Server, error)
	GetPeers() []string
	ForceBootstrap(nodeID string) error
	ForceRecoverCluster(aliveNodeIDs []string) error
	TriggerElection()
}

// StateStore defines operations for tracking memory and job counts.
type StateStore interface {
	GetMemoryUsage() int64
	GetJobCount() int64
	UpdateMemoryUsage(delta int64) error
	UpdateJobCount(delta int64) error
}

// NodeStore defines operations for node identity and event handling.
type NodeStore interface {
	GetNodeID() string
	GetRaftBind() string
	GetRaftAdvertise() string
	GetHTTPBind() string
	SetHTTPBind(httpBind string)
	SetEventHandler(handler JobEventHandler)
}

// ApplyStore defines the ability to apply a raw Raft command.
type ApplyStore interface {
	Apply(data []byte) error
}
