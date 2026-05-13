package simulator

import (
	"scheduled-db/internal/logger"
	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"
	"time"

	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type SimulatedWorker struct {
	clock    *SimulatedClock
	executor *SimulatedExecutor
	fsm      *store.FSM
	wheel    *slots.HierarchicalTimingWheel
	slotGap  time.Duration
}

func NewSimulatedWorker(clock *SimulatedClock, executor *SimulatedExecutor, fsm *store.FSM, wheel *slots.HierarchicalTimingWheel, slotGap time.Duration) *SimulatedWorker {
	return &SimulatedWorker{
		clock:    clock,
		executor: executor,
		fsm:      fsm,
		wheel:    wheel,
		slotGap:  slotGap,
	}
}

type TickResult struct {
	ExecutedJobs []ExecutionResult
	Rescheduled  []string
	ReadySlots   int64
}

func (w *SimulatedWorker) ProcessTick() TickResult {
	now := w.clock.Now()
	readyKeys := w.wheel.Advance(now)
	result := TickResult{
		ExecutedJobs: make([]ExecutionResult, 0),
		Rescheduled:  make([]string, 0),
		ReadySlots:   int64(len(readyKeys)),
	}

	for _, slotKey := range readyKeys {
		slotGapSec := int64(w.slotGap.Seconds())
		if slotGapSec == 0 {
			slotGapSec = int64(slots.DefaultSlotGap.Seconds())
		}

		slotData, exists := w.fsm.GetSlot(slotKey)
		if !exists {
			w.wheel.Remove(slotKey)
			continue
		}

		if len(slotData.JobIDs) == 0 {
			w.wheel.Remove(slotKey)
			continue
		}

		for _, jobID := range slotData.JobIDs {
			job, jobExists := w.fsm.GetJob(jobID)
			if !jobExists {
				continue
			}

			if !w.shouldExecute(job, now) {
				continue
			}

			execResult := w.executor.Execute(job, now)
			result.ExecutedJobs = append(result.ExecutedJobs, execResult)

			if job.Type == store.JobUnico {
				w.fsm.DeleteJob(jobID)
			} else if job.Type == store.JobRecurrente {
				nextTs := w.calculateNextExecution(job, now)
				if nextTs > 0 {
					w.fsm.DeleteJob(jobID)
					rescheduled := job.Clone()
					rescheduled.CreatedAt = nextTs
					w.fsm.CreateJob(rescheduled)
					w.addToWheel(rescheduled)
					result.Rescheduled = append(result.Rescheduled, jobID)
				} else {
					w.fsm.DeleteJob(jobID)
				}
			}
		}

		w.wheel.Remove(slotKey)
	}

	return result
}

func (w *SimulatedWorker) shouldExecute(job *store.Job, now int64) bool {
	if job.Type == store.JobUnico && job.Timestamp != nil {
		return *job.Timestamp <= now
	}
	if job.Type == store.JobRecurrente {
		return job.CreatedAt <= now
	}
	return false
}

func (w *SimulatedWorker) calculateNextExecution(job *store.Job, now int64) int64 {
	schedule, err := cronParser.Parse(job.CronExpr)
	if err != nil {
		logger.Debug("simulator: invalid cron expression for job %s: %v", job.ID, err)
		return 0
	}

	nextTime := schedule.Next(time.Unix(now, 0))
	nextTs := nextTime.Unix()

	if job.LastDate != nil && nextTs > *job.LastDate {
		logger.Debug("simulator: job %s reached last_date, not rescheduling", job.ID)
		return 0
	}

	return nextTs
}

func (w *SimulatedWorker) addToWheel(job *store.Job) {
	var ts int64
	if job.Type == store.JobUnico && job.Timestamp != nil {
		ts = *job.Timestamp
	} else if job.Type == store.JobRecurrente {
		ts = job.CreatedAt
	} else {
		return
	}

	slotGapSec := int64(w.slotGap.Seconds())
	if slotGapSec == 0 {
		slotGapSec = int64(slots.DefaultSlotGap.Seconds())
	}
	key := ts / slotGapSec
	w.wheel.Add(key)
}
