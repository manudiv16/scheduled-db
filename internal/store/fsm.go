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
)

type Command struct {
	Type CommandType `json:"type"`
	Job  *Job        `json:"job,omitempty"`
	ID   string      `json:"id,omitempty"`
}

// FSM implements the raft.FSM interface
type FSM struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewFSM() *FSM {
	return &FSM{
		jobs: make(map[string]*Job),
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

	return &Snapshot{jobs: jobs}, nil
}

// Restore restores the FSM state from a snapshot
func (f *FSM) Restore(reader io.ReadCloser) error {
	defer reader.Close()

	var jobs map[string]*Job
	if err := json.NewDecoder(reader).Decode(&jobs); err != nil {
		return fmt.Errorf("failed to decode snapshot: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.jobs = jobs
	log.Printf("Restored %d jobs from snapshot", len(f.jobs))
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

// Snapshot implements raft.FSMSnapshot
type Snapshot struct {
	jobs map[string]*Job
}

// Persist saves the snapshot to the given sink
func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		encoder := json.NewEncoder(sink)
		return encoder.Encode(s.jobs)
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
