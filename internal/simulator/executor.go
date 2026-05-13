package simulator

import (
	"math/rand"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/store"
)

type ExecutionResult struct {
	JobID        string
	Success      bool
	ResponseTime int64
	Timestamp    int64
}

type SimulatedExecutor struct {
	successRate float64
	latencyMs   int64
	execLog     []ExecutionResult
	logCapacity int
}

func NewSimulatedExecutor(successRate float64, latencyMs int64) *SimulatedExecutor {
	if successRate < 0 {
		successRate = 0
	}
	if successRate > 1 {
		successRate = 1
	}
	if latencyMs < 0 {
		latencyMs = 0
	}
	return &SimulatedExecutor{
		successRate: successRate,
		latencyMs:   latencyMs,
		execLog:     make([]ExecutionResult, 0, 100),
		logCapacity: 200,
	}
}

func (e *SimulatedExecutor) SetSuccessRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	e.successRate = rate
}

func (e *SimulatedExecutor) Execute(job *store.Job, now int64) ExecutionResult {
	jitter := int64(0)
	if e.latencyMs > 0 {
		jitter = rand.Int63n(e.latencyMs)
	}

	success := rand.Float64() < e.successRate

	result := ExecutionResult{
		JobID:        job.ID,
		Success:      success,
		ResponseTime: e.latencyMs + jitter,
		Timestamp:    now,
	}

	e.appendLog(result)

	if success {
		logger.Debug("simulated execution: job %s completed in %dms", job.ID, result.ResponseTime)
	} else {
		logger.Debug("simulated execution: job %s failed after %dms", job.ID, result.ResponseTime)
	}

	return result
}

func (e *SimulatedExecutor) appendLog(result ExecutionResult) {
	e.execLog = append(e.execLog, result)
	if len(e.execLog) > e.logCapacity {
		e.execLog = e.execLog[len(e.execLog)-e.logCapacity:]
	}
}

func (e *SimulatedExecutor) GetLog() []ExecutionResult {
	result := make([]ExecutionResult, len(e.execLog))
	copy(result, e.execLog)
	return result
}

func (e *SimulatedExecutor) GetLogSince(since int64) []ExecutionResult {
	for i, entry := range e.execLog {
		if entry.Timestamp >= since {
			result := make([]ExecutionResult, len(e.execLog[i:]))
			copy(result, e.execLog[i:])
			return result
		}
	}
	return nil
}

func (e *SimulatedExecutor) ClearLog() {
	e.execLog = e.execLog[:0]
}
