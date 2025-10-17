package slots

import (
	"log"
	"time"

	"scheduled-db/internal/store"

	"github.com/robfig/cron/v3"
)

type Worker struct {
	slotQueue *SlotQueue
	store     *store.Store
	stopCh    chan struct{}
	running   bool
}

func NewWorker(slotQueue *SlotQueue, store *store.Store) *Worker {
	return &Worker{
		slotQueue: slotQueue,
		store:     store,
		stopCh:    make(chan struct{}),
	}
}

func (w *Worker) Start() {
	if w.running {
		log.Println("[WORKER DEBUG] Worker already running, skipping start")
		return
	}

	w.running = true
	go w.run()
	log.Printf("[WORKER DEBUG] Worker started, goroutine launched")
}

func (w *Worker) Stop() {
	if !w.running {
		log.Println("[WORKER DEBUG] Worker already stopped, skipping stop")
		return
	}

	log.Printf("[WORKER DEBUG] Stopping worker, closing stop channel")
	close(w.stopCh)
	w.running = false
	log.Printf("[WORKER DEBUG] Worker stopped")
}

func (w *Worker) run() {
	log.Printf("[WORKER DEBUG] Worker run() started")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer log.Printf("[WORKER DEBUG] Worker run() exiting")

	for {
		select {
		case <-w.stopCh:
			log.Printf("[WORKER DEBUG] Worker received stop signal, exiting")
			return
		case <-ticker.C:
			w.processSlots()
		}
	}
}

func (w *Worker) processSlots() {
	now := time.Now().Unix()

	// Get all available slots and process them until we find one that's ready
	processedAnyJob := false
	maxSlotsToCheck := 10 // Prevent infinite loops

	for i := 0; i < maxSlotsToCheck; i++ {
		slot := w.slotQueue.GetNextSlot()
		if slot == nil {
			break
		}

		log.Printf("[WORKER DEBUG] Found slot %d with time range %d-%d, jobs count: %d",
			slot.Key, slot.MinTime, slot.MaxTime, len(slot.Jobs))

		if now < slot.MinTime {
			// Not time yet for this slot (and subsequent slots will be even later)
			log.Printf("[WORKER DEBUG] Not time yet for slot %d (now: %d < min: %d)",
				slot.Key, now, slot.MinTime)
			break
		}

		// Process jobs in this slot
		jobsToRemove := make([]string, 0)
		jobsToReschedule := make([]*store.Job, 0)
		anyJobExecuted := false
		hasJobsNotReady := false

		log.Printf("[WORKER DEBUG] Processing %d jobs in slot %d", len(slot.Jobs), slot.Key)

		for _, job := range slot.Jobs {
			log.Printf("[WORKER DEBUG] Checking job %s (type: %s)", job.ID, job.Type)

			if w.shouldExecuteJob(job, now) {
				log.Printf("[WORKER DEBUG] Executing job %s", job.ID)
				w.executeJob(job)
				anyJobExecuted = true
				processedAnyJob = true

				if job.Type == store.JobUnico {
					// Mark unique job for removal
					jobsToRemove = append(jobsToRemove, job.ID)
					log.Printf("[WORKER DEBUG] Marked unique job %s for removal", job.ID)
				} else if job.Type == store.JobRecurrente {
					// Calculate next execution time for recurring job
					nextTimestamp := w.calculateNextExecution(job, now)
					if nextTimestamp > 0 {
						// Remove from current slot and reschedule with same ID
						jobsToRemove = append(jobsToRemove, job.ID)
						job.CreatedAt = nextTimestamp
						jobsToReschedule = append(jobsToReschedule, job)
						log.Printf("[WORKER DEBUG] Rescheduled recurring job %s for %d", job.ID, nextTimestamp)
					} else {
						// Job has reached its last_date, remove it
						jobsToRemove = append(jobsToRemove, job.ID)
						log.Printf("[WORKER DEBUG] Recurring job %s reached last_date, removing", job.ID)
					}
				}
			} else {
				log.Printf("[WORKER DEBUG] Job %s should not execute yet", job.ID)
				hasJobsNotReady = true
			}
		}

		// Remove executed unique jobs and expired recurring jobs from store
		for _, jobID := range jobsToRemove {
			if w.store.IsLeader() {
				if err := w.store.DeleteJob(jobID); err != nil {
					log.Printf("Failed to delete job %s from store: %v", jobID, err)
				}
			}
			w.slotQueue.RemoveJob(jobID)
		}

		// Reschedule recurring jobs
		for _, job := range jobsToReschedule {
			if w.store.IsLeader() {
				if err := w.store.CreateJob(job); err != nil {
					log.Printf("Failed to reschedule job %s: %v", job.ID, err)
				} else {
					log.Printf("Rescheduled recurring job %s for next execution", job.ID)
				}
			}
		}

		// If no jobs were executed and there are jobs not ready, stop processing
		// This slot will remain and be checked again next tick
		if !anyJobExecuted && hasJobsNotReady {
			log.Printf("[WORKER DEBUG] Slot %d has jobs not ready yet, stopping slot processing for this tick", slot.Key)
			break
		}

		// If we executed some jobs, continue to check next slot
		// If all jobs were executed, the slot was automatically removed by RemoveJob
	}

	// If no jobs were processed in this cycle, the worker will continue checking
	// in the next iteration based on the ticker interval
	_ = processedAnyJob
}

func (w *Worker) shouldExecuteJob(job *store.Job, now int64) bool {
	log.Printf("[WORKER DEBUG] shouldExecuteJob: job %s, type '%s', now %d", job.ID, job.Type, now)
	log.Printf("[WORKER DEBUG] Job type comparison: job.Type == store.JobUnico: %v, job.Type == store.JobRecurrente: %v", job.Type == store.JobUnico, job.Type == store.JobRecurrente)

	if job.Type == store.JobUnico && job.Timestamp != nil {
		shouldExecute := *job.Timestamp <= now
		log.Printf("[WORKER DEBUG] Job %s (unique): timestamp %d <= now %d = %v", job.ID, *job.Timestamp, now, shouldExecute)
		return shouldExecute
	}

	log.Printf("[WORKER DEBUG] About to check if job.Type == store.JobRecurrente")
	if job.Type == store.JobRecurrente {
		log.Printf("[WORKER DEBUG] Inside JobRecurrente branch")
		// For recurring jobs, we need to check if it's time based on cron
		shouldExecute := w.isTimeForRecurringJob(job, now)
		log.Printf("[WORKER DEBUG] Job %s (recurring): shouldExecute = %v", job.ID, shouldExecute)
		return shouldExecute
	}

	log.Printf("[WORKER DEBUG] Job %s: invalid type or missing timestamp", job.ID)
	return false
}

func (w *Worker) isTimeForRecurringJob(job *store.Job, now int64) bool {
	// For recurring jobs, the CreatedAt time represents when this instance should run
	shouldExecute := job.CreatedAt <= now
	log.Printf("[WORKER DEBUG] isTimeForRecurringJob: job %s, CreatedAt %d, now %d, shouldExecute %v", job.ID, job.CreatedAt, now, shouldExecute)
	return shouldExecute
}

func (w *Worker) executeJob(job *store.Job) {
	log.Printf("executed job id:%s", job.ID)
}

func (w *Worker) calculateNextExecution(job *store.Job, now int64) int64 {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(job.CronExpr)
	if err != nil {
		log.Printf("Invalid cron expression for job %s: %v", job.ID, err)
		return 0
	}

	nextTime := schedule.Next(time.Unix(now, 0))
	nextTimestamp := nextTime.Unix()

	// Check if next execution exceeds last_date
	if job.LastDate != nil && nextTimestamp > *job.LastDate {
		log.Printf("Job %s reached last_date, not rescheduling", job.ID)
		return 0
	}

	return nextTimestamp
}
