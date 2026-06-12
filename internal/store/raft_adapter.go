//go:build !wasm

package store

import (
	"encoding/json"
	"fmt"
	"io"

	"scheduled-db/internal/logger"

	"github.com/hashicorp/raft"
)

// Apply applies a Raft log entry to the FSM
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(logEntry.Data, &cmd); err != nil {
		logger.Debug("Failed to unmarshal command: %v", err)
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	return f.applyCommandInternal(&cmd)
}

// Snapshot creates a Raft snapshot of the FSM state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return f.GetSnapshot(), nil
}

// Restore restores the FSM state from a Raft snapshot
func (f *FSM) Restore(reader io.ReadCloser) error {
	defer reader.Close()

	var snapshot struct {
		Jobs            map[string]*Job               `json:"jobs"`
		Slots           map[int64]*SlotData           `json:"slots"`
		ColdSlotMeta    map[int64]bool                `json:"cold_slot_meta"`
		ExecutionStates map[string]*JobExecutionState `json:"execution_states"`
		MemoryUsage     int64                         `json:"memory_usage"`
		JobCount        int64                         `json:"job_count"`
	}

	if err := json.NewDecoder(reader).Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.jobs = snapshot.Jobs
	if f.jobs == nil {
		f.jobs = make(map[string]*Job)
	}

	f.slots = snapshot.Slots
	if f.slots == nil {
		f.slots = make(map[int64]*SlotData)
	}

	f.coldSlotMeta = snapshot.ColdSlotMeta
	if f.coldSlotMeta == nil {
		f.coldSlotMeta = make(map[int64]bool)
	}

	f.executionStates = snapshot.ExecutionStates
	if f.executionStates == nil {
		f.executionStates = make(map[string]*JobExecutionState)
	}

	f.memoryUsage.Store(snapshot.MemoryUsage)
	f.jobCount.Store(snapshot.JobCount)

	logger.Debug("Restored %d jobs, %d slots, %d cold slots, %d execution states, %d bytes memory, %d job count from snapshot",
		len(f.jobs), len(f.slots), len(f.coldSlotMeta), len(f.executionStates), snapshot.MemoryUsage, snapshot.JobCount)
	return nil
}

// Persist persists the snapshot to a Raft sink
func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		encoder := json.NewEncoder(sink)
		data := struct {
			Jobs            map[string]*Job               `json:"jobs"`
			Slots           map[int64]*SlotData           `json:"slots"`
			ColdSlotMeta    map[int64]bool                `json:"cold_slot_meta"`
			ExecutionStates map[string]*JobExecutionState `json:"execution_states"`
			MemoryUsage     int64                         `json:"memory_usage"`
			JobCount        int64                         `json:"job_count"`
		}{
			Jobs:            s.jobs,
			Slots:           s.slots,
			ColdSlotMeta:    s.coldSlotMeta,
			ExecutionStates: s.executionStates,
			MemoryUsage:     s.memoryUsage,
			JobCount:        s.jobCount,
		}
		return encoder.Encode(data)
	}()

	if err != nil {
		if err := sink.Cancel(); err != nil {
			logger.Debug("Failed to cancel sink: %v", err)
		}
		return err
	}

	return sink.Close()
}

// Release is called when the snapshot is no longer needed
func (s *Snapshot) Release() {}
