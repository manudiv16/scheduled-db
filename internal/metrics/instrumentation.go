package metrics

import (
	"context"
	"sync"
	"time"

	"scheduled-db/internal/store"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// GlobalMetrics holds the global metrics instance
var (
	globalMetrics *Metrics
	globalMutex   sync.RWMutex
)

// Initialize sets up the global metrics instance
func Initialize() error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	var err error
	globalMetrics, err = NewMetrics()
	return err
}

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return globalMetrics
}

// JobInstrumentation provides instrumentation helpers for job operations
type JobInstrumentation struct {
	metrics *Metrics
}

// NewJobInstrumentation creates a new job instrumentation helper
func NewJobInstrumentation(metrics *Metrics) *JobInstrumentation {
	return &JobInstrumentation{
		metrics: metrics,
	}
}

// RecordJobCreated records metrics when a job is created
func (ji *JobInstrumentation) RecordJobCreated(ctx context.Context, job *store.Job) {
	if ji.metrics == nil {
		return
	}
	ji.metrics.IncrementJobsCreated(ctx, string(job.Type))
}

// RecordJobDeleted records metrics when a job is deleted
func (ji *JobInstrumentation) RecordJobDeleted(ctx context.Context, job *store.Job) {
	if ji.metrics == nil {
		return
	}
	ji.metrics.IncrementJobsDeleted(ctx, string(job.Type))
}

// RecordJobExecution records metrics for job execution
func (ji *JobInstrumentation) RecordJobExecution(ctx context.Context, job *store.Job, duration time.Duration, success bool) {
	if ji.metrics == nil {
		return
	}
	ji.metrics.IncrementJobsExecuted(ctx, string(job.Type), success)
	ji.metrics.RecordJobExecutionDuration(ctx, duration, string(job.Type))
}

// UpdateJobCounts updates the current job counts
func (ji *JobInstrumentation) UpdateJobCounts(ctx context.Context, jobs map[string]*store.Job) {
	if ji.metrics == nil || jobs == nil {
		return
	}

	var totalActive int64
	uniqueCount := int64(0)
	recurrentCount := int64(0)

	for _, job := range jobs {
		totalActive++
		switch job.Type {
		case store.JobUnico:
			uniqueCount++
		case store.JobRecurrente:
			recurrentCount++
		}
	}

	ji.metrics.SetActiveJobs(ctx, totalActive)

	// Reset and set job counts by type
	ji.metrics.JobsByType.Add(ctx, uniqueCount, metric.WithAttributes(
		attribute.String("job_type", "unico"),
	))
	ji.metrics.JobsByType.Add(ctx, recurrentCount, metric.WithAttributes(
		attribute.String("job_type", "recurrente"),
	))
}

// SlotInstrumentation provides instrumentation helpers for slot operations
type SlotInstrumentation struct {
	metrics *Metrics
}

// NewSlotInstrumentation creates a new slot instrumentation helper
func NewSlotInstrumentation(metrics *Metrics) *SlotInstrumentation {
	return &SlotInstrumentation{
		metrics: metrics,
	}
}

// RecordSlotCreated records metrics when a slot is created
func (si *SlotInstrumentation) RecordSlotCreated(ctx context.Context) {
	if si.metrics == nil {
		return
	}
	si.metrics.IncrementSlotsCreated(ctx)
}

// RecordSlotDeleted records metrics when a slot is deleted
func (si *SlotInstrumentation) RecordSlotDeleted(ctx context.Context) {
	if si.metrics == nil {
		return
	}
	si.metrics.IncrementSlotsDeleted(ctx)
}

// RecordSlotProcessed records metrics when a slot is processed
func (si *SlotInstrumentation) RecordSlotProcessed(ctx context.Context, jobCount int64, duration time.Duration) {
	if si.metrics == nil {
		return
	}
	si.metrics.IncrementSlotsProcessed(ctx, jobCount)
	si.metrics.RecordSlotProcessingDuration(ctx, duration)
}

// UpdateSlotCounts updates the current slot counts
func (si *SlotInstrumentation) UpdateSlotCounts(ctx context.Context, activeSlots int64) {
	if si.metrics == nil {
		return
	}
	si.metrics.SetActiveSlots(ctx, activeSlots)
}

// WorkerInstrumentation provides instrumentation helpers for worker operations
type WorkerInstrumentation struct {
	metrics   *Metrics
	startTime time.Time
	isActive  bool
}

// NewWorkerInstrumentation creates a new worker instrumentation helper
func NewWorkerInstrumentation(metrics *Metrics) *WorkerInstrumentation {
	return &WorkerInstrumentation{
		metrics: metrics,
	}
}

// RecordWorkerStarted records when worker starts
func (wi *WorkerInstrumentation) RecordWorkerStarted(ctx context.Context) {
	if wi.metrics == nil {
		return
	}
	wi.startTime = time.Now()
	wi.isActive = true
	wi.metrics.SetWorkerActive(ctx, true)
}

// RecordWorkerStopped records when worker stops
func (wi *WorkerInstrumentation) RecordWorkerStopped(ctx context.Context) {
	if wi.metrics == nil {
		return
	}
	if wi.isActive {
		wi.isActive = false
		wi.metrics.SetWorkerActive(ctx, false)
	}
}

// RecordProcessingCycle records a complete worker processing cycle
func (wi *WorkerInstrumentation) RecordProcessingCycle(ctx context.Context, processingDuration, idleDuration time.Duration) {
	if wi.metrics == nil {
		return
	}
	wi.metrics.RecordWorkerProcessingTime(ctx, processingDuration)
	wi.metrics.RecordWorkerIdleTime(ctx, idleDuration)
}

// RecordWorkerError records a worker error
func (wi *WorkerInstrumentation) RecordWorkerError(ctx context.Context, errorType string) {
	if wi.metrics == nil {
		return
	}
	wi.metrics.IncrementWorkerErrors(ctx, errorType)
}

// ClusterInstrumentation provides instrumentation helpers for cluster operations
type ClusterInstrumentation struct {
	metrics       *Metrics
	currentLeader string
	currentNodes  int64
	nodeID        string
}

// NewClusterInstrumentation creates a new cluster instrumentation helper
func NewClusterInstrumentation(metrics *Metrics, nodeID string) *ClusterInstrumentation {
	return &ClusterInstrumentation{
		metrics: metrics,
		nodeID:  nodeID,
	}
}

// RecordLeaderChange records a leader change event
func (ci *ClusterInstrumentation) RecordLeaderChange(ctx context.Context, oldLeader, newLeader string) {
	if ci.metrics == nil {
		return
	}
	ci.metrics.IncrementLeaderChanges(ctx, oldLeader, newLeader)
	ci.metrics.IncrementLeaderElections(ctx)
	ci.currentLeader = newLeader
}

// RecordNodeJoin records a node join event
func (ci *ClusterInstrumentation) RecordNodeJoin(ctx context.Context, nodeID string) {
	if ci.metrics == nil {
		return
	}
	ci.metrics.IncrementClusterJoin(ctx, nodeID)
	ci.currentNodes++
	ci.metrics.SetClusterNodes(ctx, ci.currentNodes)
}

// RecordNodeLeave records a node leave event
func (ci *ClusterInstrumentation) RecordNodeLeave(ctx context.Context, nodeID string) {
	if ci.metrics == nil {
		return
	}
	ci.metrics.IncrementClusterLeave(ctx, nodeID)
	if ci.currentNodes > 0 {
		ci.currentNodes--
	}
	ci.metrics.SetClusterNodes(ctx, ci.currentNodes)
}

// RecordRaftOperation records a Raft operation
func (ci *ClusterInstrumentation) RecordRaftOperation(ctx context.Context, operation string, success bool) {
	if ci.metrics == nil {
		return
	}
	ci.metrics.IncrementRaftOperation(ctx, operation, success)
}

// RecordSplitBrainEvent records a split-brain prevention event
func (ci *ClusterInstrumentation) RecordSplitBrainEvent(ctx context.Context) {
	if ci.metrics == nil {
		return
	}
	ci.metrics.IncrementSplitBrainEvents(ctx)
}

// UpdateClusterState updates the current cluster state metrics
func (ci *ClusterInstrumentation) UpdateClusterState(ctx context.Context, nodeCount int64, leader string) {
	if ci.metrics == nil {
		return
	}

	// Check for leader change
	if ci.currentLeader != leader && leader != "" {
		ci.RecordLeaderChange(ctx, ci.currentLeader, leader)
	}

	ci.currentNodes = nodeCount
	ci.metrics.SetClusterNodes(ctx, nodeCount)
}

// DiscoveryInstrumentation provides instrumentation helpers for discovery operations
type DiscoveryInstrumentation struct {
	metrics  *Metrics
	strategy string
}

// NewDiscoveryInstrumentation creates a new discovery instrumentation helper
func NewDiscoveryInstrumentation(metrics *Metrics, strategy string) *DiscoveryInstrumentation {
	return &DiscoveryInstrumentation{
		metrics:  metrics,
		strategy: strategy,
	}
}

// RecordDiscoveryEvent records a discovery event
func (di *DiscoveryInstrumentation) RecordDiscoveryEvent(ctx context.Context, eventType string) {
	if di.metrics == nil {
		return
	}
	di.metrics.IncrementDiscoveryEvent(ctx, di.strategy, eventType)
}

// RecordDiscoveryError records a discovery error
func (di *DiscoveryInstrumentation) RecordDiscoveryError(ctx context.Context, errorType string) {
	if di.metrics == nil {
		return
	}
	di.metrics.IncrementDiscoveryError(ctx, di.strategy, errorType)
}

// RecordDiscoveryOperation records a discovery operation
func (di *DiscoveryInstrumentation) RecordDiscoveryOperation(ctx context.Context, operation string, success bool) {
	if di.metrics == nil {
		return
	}
	di.metrics.IncrementDiscoveryOperation(ctx, di.strategy, operation, success)
}

// UpdateNodesDiscovered updates the count of discovered nodes
func (di *DiscoveryInstrumentation) UpdateNodesDiscovered(ctx context.Context, count int64) {
	if di.metrics == nil {
		return
	}
	di.metrics.SetNodesDiscovered(ctx, count, di.strategy)
}

// WebhookInstrumentation provides instrumentation helpers for webhook operations
type WebhookInstrumentation struct {
	metrics *Metrics
}

// NewWebhookInstrumentation creates a new webhook instrumentation helper
func NewWebhookInstrumentation(metrics *Metrics) *WebhookInstrumentation {
	return &WebhookInstrumentation{
		metrics: metrics,
	}
}

// RecordWebhookRequest records a webhook request
func (wi *WebhookInstrumentation) RecordWebhookRequest(ctx context.Context, duration time.Duration, success bool) {
	if wi.metrics == nil {
		return
	}
	wi.metrics.IncrementWebhookRequests(ctx, success)
	wi.metrics.RecordWebhookDuration(ctx, duration, success)
}

// SystemInstrumentation provides instrumentation helpers for system metrics
type SystemInstrumentation struct {
	metrics   *Metrics
	startTime time.Time
}

// NewSystemInstrumentation creates a new system instrumentation helper
func NewSystemInstrumentation(metrics *Metrics) *SystemInstrumentation {
	startTime := time.Now()
	if metrics != nil {
		ctx := context.Background()
		metrics.SetSystemStartTime(ctx, startTime)
	}

	return &SystemInstrumentation{
		metrics:   metrics,
		startTime: startTime,
	}
}

// UpdateUptime updates the system uptime metric
func (si *SystemInstrumentation) UpdateUptime(ctx context.Context) {
	if si.metrics == nil {
		return
	}
	si.metrics.UpdateSystemUptime(ctx, si.startTime)
}

// GetStartTime returns the system start time
func (si *SystemInstrumentation) GetStartTime() time.Time {
	return si.startTime
}

// Global instrumentation instances for easy access
var (
	GlobalJobInstrumentation       *JobInstrumentation
	GlobalSlotInstrumentation      *SlotInstrumentation
	GlobalWorkerInstrumentation    *WorkerInstrumentation
	GlobalClusterInstrumentation   *ClusterInstrumentation
	GlobalDiscoveryInstrumentation *DiscoveryInstrumentation
	GlobalWebhookInstrumentation   *WebhookInstrumentation
	GlobalSystemInstrumentation    *SystemInstrumentation
)

// InitializeGlobalInstrumentation initializes all global instrumentation helpers
func InitializeGlobalInstrumentation(nodeID string, discoveryStrategy string) error {
	if err := Initialize(); err != nil {
		return err
	}

	metrics := GetMetrics()
	GlobalJobInstrumentation = NewJobInstrumentation(metrics)
	GlobalSlotInstrumentation = NewSlotInstrumentation(metrics)
	GlobalWorkerInstrumentation = NewWorkerInstrumentation(metrics)
	GlobalClusterInstrumentation = NewClusterInstrumentation(metrics, nodeID)
	GlobalDiscoveryInstrumentation = NewDiscoveryInstrumentation(metrics, discoveryStrategy)
	GlobalWebhookInstrumentation = NewWebhookInstrumentation(metrics)
	GlobalSystemInstrumentation = NewSystemInstrumentation(metrics)

	return nil
}
