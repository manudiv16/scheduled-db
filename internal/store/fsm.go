package store

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/hashicorp/raft"
)

type CommandType string

const (
	CommandCreateJob CommandType = "create_job"
	CommandDeleteJob CommandType = "delete_job"
	CommandCreateSlot CommandType = "create_slot"
	CommandDeleteSlot CommandType = "delete_slot"
)

type Command struct {
	Type CommandType `json:"type"`
	Job  *Job        `json:"job,omitempty"`
	ID   string      `json:"id,omitempty"`
	Slot *SlotData   `json:"slot,omitempty"`
}

// FSM implements the raft.FSM interface
type FSM struct {
	mu    sync.RWMutex
	jobs  map[string]*Job
	slots map[int64]*SlotData
}

func NewFSM() *FSM {
	return &FSM{
		jobs:  make(map[string]*Job),
		slots: make(map[int64]*SlotData),
	}
}

// Apply applies a Raft log entry to the FSM
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(logEntry.Data, &cmd); err != nil {
		log.Printf("Failed to unmarshal command: %v", err)
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch cmd.Type {
	case CommandCreateJob:
		if cmd.Job == nil {
			return fmt.Errorf("job is required for create command")
		}
		f.jobs[cmd.Job.ID] = cmd.Job
		log.Printf("Created job %s in FSM", cmd.Job.ID)
		return cmd.Job
	case CommandDeleteJob:
		if cmd.ID == "" {
			return fmt.Errorf("job ID is required for delete command")
		}
		delete(f.jobs, cmd.ID)
		log.Printf("Deleted job %s from FSM", cmd.ID)
		return nil
	case CommandCreateSlot:
		if cmd.Slot == nil {
			return fmt.Errorf("slot is required for create slot command")
		}
		f.slots[cmd.Slot.Key] = cmd.Slot
		log.Printf("Created slot %d in FSM", cmd.Slot.Key)
		return cmd.Slot
	case CommandDeleteSlot:
		if cmd.ID == "" {
			return fmt.Errorf("slot key is required for delete slot command")
		}
		// Parse key from string
		var key int64
		if _, err := fmt.Sscanf(cmd.ID, "%d", &key); err != nil {
			return fmt.Errorf("invalid slot key: %s", cmd.ID)
		}
		delete(f.slots, key)
		log.Printf("Deleted slot %d from FSM", key)
		return nil
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// Snapshot creates a snapshot of the FSM state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create a copy of the jobs map
	jobs := make(map[string]*Job)
	for k, v := range f.jobs {
		jobs[k] = v
	}

	// Create a copy of the slots map
	slots := make(map[int64]*SlotData)
	for k, v := range f.slots {
		slots[k] = v
	}

	return &Snapshot{jobs: jobs, slots: slots}, nil
}

// Restore restores the FSM state from a snapshot
func (f *FSM) Restore(reader io.ReadCloser) error {
	defer reader.Close()

	var snapshot struct {
		Jobs  map[string]*Job      `json:"jobs"`
		Slots map[int64]*SlotData  `json:"slots"`
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
	
	log.Printf("Restored %d jobs and %d slots from snapshot", len(f.jobs), len(f.slots))
	return nil
}

// GetJob returns a job by ID
func (f *FSM) GetJob(id string) (*Job, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	job, exists := f.jobs[id]
	return job, exists
}

// GetAllJobs returns all jobs
func (f *FSM) GetAllJobs() map[string]*Job {
	f.mu.RLock()
	defer f.mu.RUnlock()

	jobs := make(map[string]*Job)
	for k, v := range f.jobs {
		jobs[k] = v
	}
	return jobs
}

// GetSlot returns a slot by key
func (f *FSM) GetSlot(key int64) (*SlotData, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	slot, exists := f.slots[key]
	return slot, exists
}

// GetAllSlots returns all slots
func (f *FSM) GetAllSlots() map[int64]*SlotData {
	f.mu.RLock()
	defer f.mu.RUnlock()

	slots := make(map[int64]*SlotData)
	for k, v := range f.slots {
		slots[k] = v
	}
	return slots
}

// Snapshot implements raft.FSMSnapshot
type Snapshot struct {
	jobs  map[string]*Job
	slots map[int64]*SlotData
}

// Persist saves the snapshot to the given sink
func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		encoder := json.NewEncoder(sink)
		data := struct {
			Jobs  map[string]*Job      `json:"jobs"`
			Slots map[int64]*SlotData  `json:"slots"`
		}{
			Jobs:  s.jobs,
			Slots: s.slots,
		}
		return encoder.Encode(data)
	}()

	if err != nil {
		if err := sink.Cancel(); err != nil {
			log.Printf("Failed to cancel sink: %v", err)
		}
		return err
	}

	return sink.Close()
}

// Release is called when the snapshot is no longer needed
func (s *Snapshot) Release() {}
