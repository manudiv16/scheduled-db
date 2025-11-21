package slots

import (
	"context"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/metrics"
	"scheduled-db/internal/store"

	"github.com/robfig/cron/v3"
)

type Worker struct {
	slotQueue         *PersistentSlotQueue
	store             *store.Store
	executionManager  *ExecutionManager
	stopCh            chan struct{}
	running           bool
	inProgressTimeout time.Duration
}

func NewWorker(slotQueue *PersistentSlotQueue, store *store.Store, executionManager *ExecutionManager, inProgressTimeout time.Duration) *Worker {
	return &Worker{
		slotQueue:         slotQueue,
		store:             store,
		executionManager:  executionManager,
		stopCh:            make(chan struct{}),
		inProgressTimeout: inProgressTimeout,
	}
}

func (w *Worker) Start() {
	if w.running {
		logger.Debug("worker already running, skipping start")
		return
	}

	w.running = true
	go w.run()
	logger.Debug("worker started")
}

func (w *Worker) Stop() {
	if !w.running {
		logger.Debug("worker already stopped, skipping stop")
		return
	}

	logger.Debug("stopping worker")
	close(w.stopCh)
	w.running = false
	logger.Debug("worker stopped")
}

func (w *Worker) run() {
	logger.Debug("worker run() started")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer logger.Debug("worker run() exiting")

	// Timeout checker runs every 30 seconds
	timeoutTicker := time.NewTicker(30 * time.Second)
	defer timeoutTicker.Stop()

	for {
		select {
		case <-w.stopCh:
			logger.Debug("worker received stop signal, exiting")
			return
		case <-ticker.C:
			w.processSlots()
		case <-timeoutTicker.C:
			w.checkTimeouts()
		}
	}
}

func (w *Worker) processSlots() {
	start := time.Now()
	now := start.Unix()

	// Get all available slots and process them until we find one that's ready
	maxSlotsToCheck := 10 // Prevent infinite loops

	for i := 0; i < maxSlotsToCheck; i++ {
		slot := w.slotQueue.GetNextSlot()
		if slot == nil {
			break
		}

		logger.Debug("processing slot %d with %d jobs", slot.Key, len(slot.Jobs))

		if now < slot.MinTime {
			// Not time yet for this slot (and subsequent slots will be even later)
			logger.Debug("slot %d not ready yet (now: %d < min: %d)", slot.Key, now, slot.MinTime)
			break
		}

		if len(slot.Jobs) > 0 {
			logger.Info("processing slot %d with %d jobs (ready at %d, now %d)", slot.Key, len(slot.Jobs), slot.MinTime, now)
		}

		// Record slot processing metrics using OpenTelemetry
		if metrics.GlobalSlotInstrumentation != nil {
			metrics.GlobalSlotInstrumentation.RecordSlotProcessed(context.Background(), int64(len(slot.Jobs)), time.Since(start))
		}

		// Process jobs in this slot
		jobsToRemove := make([]string, 0)
		jobsToReschedule := make([]*store.Job, 0)
		anyJobExecuted := false
		hasJobsNotReady := false

		for _, job := range slot.Jobs {
			if w.shouldExecuteJob(job, now) {
				w.executeJob(job)
				anyJobExecuted = true

				if job.Type == store.JobUnico {
					// Mark unique job for removal
					jobsToRemove = append(jobsToRemove, job.ID)
				} else if job.Type == store.JobRecurrente {
					// Calculate next execution time for recurring job
					nextTimestamp := w.calculateNextExecution(job, now)
					if nextTimestamp > 0 {
						// Remove from current slot and reschedule with same ID
						jobsToRemove = append(jobsToRemove, job.ID)
						job.CreatedAt = nextTimestamp
						jobsToReschedule = append(jobsToReschedule, job)
						logger.Debug("rescheduled recurring job %s for %d", job.ID, nextTimestamp)
					} else {
						// Job has reached its last_date, remove it
						jobsToRemove = append(jobsToRemove, job.ID)
						logger.Debug("recurring job %s reached last_date, removing", job.ID)
					}
				}
			} else {
				hasJobsNotReady = true
			}
		}

		// Remove executed unique jobs and expired recurring jobs from store
		for _, jobID := range jobsToRemove {
			if w.store.IsLeader() {
				if err := w.store.DeleteJob(jobID); err != nil {
					logger.JobError(jobID, "failed to delete from store: %v", err)
				}
			}
			w.slotQueue.RemoveJob(jobID)
		}

		// Reschedule recurring jobs
		for _, job := range jobsToReschedule {
			if w.store.IsLeader() {
				if err := w.store.CreateJob(job); err != nil {
					logger.JobError(job.ID, "failed to reschedule: %v", err)
				}
			}
		}

		// If no jobs were executed and there are jobs not ready, stop processing
		// This slot will remain and be checked again next tick
		if !anyJobExecuted && hasJobsNotReady {
			logger.Debug("slot %d has jobs not ready yet, stopping processing", slot.Key)
			break
		}
	}
}

func (w *Worker) shouldExecuteJob(job *store.Job, now int64) bool {
	if job.Type == store.JobUnico && job.Timestamp != nil {
		return *job.Timestamp <= now
	}

	if job.Type == store.JobRecurrente {
		return w.isTimeForRecurringJob(job, now)
	}

	logger.JobWarn(job.ID, "invalid type or missing timestamp")
	return false
}

func (w *Worker) isTimeForRecurringJob(job *store.Job, now int64) bool {
	// For recurring jobs, the CreatedAt time represents when this instance should run
	return job.CreatedAt <= now
}

func (w *Worker) executeJob(job *store.Job) {
	start := time.Now()
	logger.Info("executing job %s", job.ID)

	// Execute job through ExecutionManager (handles idempotency and status tracking)
	err := w.executionManager.Execute(job)
	success := err == nil

	// Record metrics
	duration := time.Since(start)
	if metrics.GlobalJobInstrumentation != nil {
		metrics.GlobalJobInstrumentation.RecordJobExecution(context.Background(), job, duration, success)
	}

	// Record worker processing metrics using OpenTelemetry
	if metrics.GlobalWorkerInstrumentation != nil {
		if !success {
			metrics.GlobalWorkerInstrumentation.RecordWorkerError(context.Background(), "job_execution_failed")
		}
		metrics.GlobalWorkerInstrumentation.RecordProcessingCycle(context.Background(), duration, 0)
	}

	if success {
		logger.JobExecuted()
	} else {
		logger.JobError(job.ID, "execution failed: %v", err)
	}
}

func (w *Worker) calculateNextExecution(job *store.Job, now int64) int64 {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(job.CronExpr)
	if err != nil {
		logger.JobError(job.ID, "invalid cron expression: %v", err)
		return 0
	}

	nextTime := schedule.Next(time.Unix(now, 0))
	nextTimestamp := nextTime.Unix()

	// Check if next execution exceeds last_date
	if job.LastDate != nil && nextTimestamp > *job.LastDate {
		logger.Debug("job %s reached last_date, not rescheduling", job.ID)
		return 0
	}

	return nextTimestamp
}

func (w *Worker) checkTimeouts() {
	if !w.store.IsLeader() {
		return
	}

	timedOutJobs, err := w.executionManager.statusTracker.CheckTimeouts(w.inProgressTimeout)
	if err != nil {
		logger.Error("failed to check timeouts: %v", err)
		return
	}

	if len(timedOutJobs) > 0 {
		logger.Info("found %d timed-out jobs", len(timedOutJobs))
	}
}
