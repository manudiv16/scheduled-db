package simulator

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"
)

type Config struct {
	SlotGap      time.Duration
	SuccessRate  float64
	LatencyMs    int64
	WheelConfigs []slots.WheelLevelConfig
}

func DefaultConfig() Config {
	return Config{
		SlotGap:     slots.DefaultSlotGap,
		SuccessRate: 0.95,
		LatencyMs:   100,
	}
}

type Simulator struct {
	mu       sync.RWMutex
	fsm      *store.FSM
	clock    *SimulatedClock
	wheel    *slots.HierarchicalTimingWheel
	worker   *SimulatedWorker
	executor *SimulatedExecutor
	config   Config
	tickLog  []TickResult
}

func NewSimulator(cfg Config) *Simulator {
	if cfg.SlotGap == 0 {
		cfg.SlotGap = slots.DefaultSlotGap
	}
	if cfg.SuccessRate == 0 {
		cfg.SuccessRate = 0.95
	}

	fsm := store.NewFSM()
	now := time.Now().Unix()
	clock := NewSimulatedClock(now)

	configs := cfg.WheelConfigs
	if len(configs) == 0 {
		configs = slots.DefaultWheelConfigs(cfg.SlotGap)
	}
	wheel := slots.NewHierarchicalTimingWheel(cfg.SlotGap, configs)
	wheel.Init(now)

	executor := NewSimulatedExecutor(cfg.SuccessRate, cfg.LatencyMs)
	worker := NewSimulatedWorker(clock, executor, fsm, wheel, cfg.SlotGap)

	return &Simulator{
		fsm:      fsm,
		clock:    clock,
		wheel:    wheel,
		worker:   worker,
		executor: executor,
		config:   cfg,
		tickLog:  make([]TickResult, 0, 100),
	}
}

func (s *Simulator) CreateJob(req store.CreateJobRequest) (*store.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, err := req.ToJob()
	if err != nil {
		return nil, fmt.Errorf("invalid job request: %w", err)
	}

	if err := s.fsm.CreateJob(job); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	s.addToWheel(job)
	logger.Info("simulator: created job %s (type=%s)", job.ID, job.Type)
	return job, nil
}

func (s *Simulator) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.fsm.DeleteJob(id); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	logger.Info("simulator: deleted job %s", id)
	return nil
}

func (s *Simulator) Tick() TickResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := s.worker.ProcessTick()

	s.tickLog = append(s.tickLog, result)
	if len(s.tickLog) > 200 {
		s.tickLog = s.tickLog[len(s.tickLog)-200:]
	}

	return result
}

func (s *Simulator) AdvanceTime(seconds float64) int64 {
	return s.clock.Tick(seconds)
}

func (s *Simulator) SetClockSpeed(speed float64) {
	s.clock.SetSpeed(speed)
}

func (s *Simulator) GetClockSpeed() float64 {
	return s.clock.GetSpeed()
}

func (s *Simulator) SetTime(t int64) {
	s.clock.SetTime(t)
}

func (s *Simulator) GetNow() int64 {
	return s.clock.Now()
}

func (s *Simulator) SetSuccessRate(rate float64) {
	s.executor.SetSuccessRate(rate)
}

type SimState struct {
	Now              int64                              `json:"now"`
	ClockSpeed       float64                            `json:"clock_speed"`
	Jobs             map[string]*store.Job              `json:"jobs"`
	Slots            map[int64]*store.SlotData          `json:"slots"`
	ExecutionStates  map[string]*store.JobExecutionState `json:"execution_states"`
	WheelSize        int                                `json:"wheel_size"`
	ExecLog          []ExecutionResult                  `json:"exec_log"`
	TickCount        int                                `json:"tick_count"`
	TotalExecuted    int                                `json:"total_executed"`
	TotalRescheduled int                                `json:"total_rescheduled"`
}

func (s *Simulator) GetState() SimState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := s.fsm.GetSnapshot()

	totalExecuted := 0
	totalRescheduled := 0
	for _, tr := range s.tickLog {
		totalExecuted += len(tr.ExecutedJobs)
		totalRescheduled += len(tr.Rescheduled)
	}

	return SimState{
		Now:              s.clock.Now(),
		ClockSpeed:       s.clock.GetSpeed(),
		Jobs:             snap.Jobs(),
		Slots:            snap.Slots(),
		ExecutionStates:  snap.ExecutionStates(),
		WheelSize:        s.wheel.Size(),
		ExecLog:          s.executor.GetLog(),
		TickCount:        len(s.tickLog),
		TotalExecuted:    totalExecuted,
		TotalRescheduled: totalRescheduled,
	}
}

func (s *Simulator) GetStateJSON() string {
	state := s.GetState()
	data, err := json.Marshal(state)
	if err != nil {
		logger.Error("simulator: failed to marshal state: %v", err)
		return "{}"
	}
	return string(data)
}

func (s *Simulator) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	s.fsm = store.NewFSM()
	s.clock.SetTime(now)
	s.wheel.Reset()
	s.wheel.Init(now)
	s.executor.ClearLog()
	s.tickLog = s.tickLog[:0]
	s.worker = NewSimulatedWorker(s.clock, s.executor, s.fsm, s.wheel, s.config.SlotGap)
	logger.Info("simulator: reset")
}

func (s *Simulator) addToWheel(job *store.Job) {
	var ts int64
	if job.Type == store.JobUnico && job.Timestamp != nil {
		ts = *job.Timestamp
	} else if job.Type == store.JobRecurrente {
		ts = job.CreatedAt
	} else {
		return
	}

	slotGapSec := int64(s.config.SlotGap.Seconds())
	if slotGapSec == 0 {
		slotGapSec = int64(slots.DefaultSlotGap.Seconds())
	}
	key := ts / slotGapSec

	slotData, exists := s.fsm.GetSlot(key)
	if !exists {
		slotData = &store.SlotData{
			Key:     key,
			MinTime: key * slotGapSec,
			MaxTime: (key+1)*slotGapSec - 1,
			JobIDs:  []string{job.ID},
		}
	} else {
		slotData.JobIDs = append(slotData.JobIDs, job.ID)
	}

	s.fsm.CreateSlot(slotData)
	s.wheel.Add(key)
}
