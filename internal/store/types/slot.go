package types

// SlotData represents a time slot persisted in Raft.
type SlotData struct {
	Key     int64    `json:"key"`
	MinTime int64    `json:"min_time"`
	MaxTime int64    `json:"max_time"`
	JobIDs  []string `json:"job_ids"`
}

// Clone returns a deep copy of the SlotData.
func (s *SlotData) Clone() *SlotData {
	if s == nil {
		return nil
	}
	clone := *s
	if s.JobIDs != nil {
		clone.JobIDs = make([]string, len(s.JobIDs))
		copy(clone.JobIDs, s.JobIDs)
	}
	return &clone
}
