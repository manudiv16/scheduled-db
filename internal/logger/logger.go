package logger

import (
	"log"
	"os"
	"sync/atomic"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

type Logger struct {
	level       Level
	jobMetrics  *JobMetrics
	lastMetrics time.Time
}

type JobMetrics struct {
	executed int64
	failed   int64
}

var globalLogger *Logger

func init() {
	globalLogger = &Logger{
		level:       INFO,
		jobMetrics:  &JobMetrics{},
		lastMetrics: time.Now(),
	}
	
	// Set log level from environment
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		switch level {
		case "DEBUG":
			globalLogger.level = DEBUG
		case "INFO":
			globalLogger.level = INFO
		case "WARN":
			globalLogger.level = WARN
		case "ERROR":
			globalLogger.level = ERROR
		}
	}
}

func (l *Logger) shouldLog(level Level) bool {
	return level >= l.level
}

// ERROR - Fallos críticos
func Error(msg string, args ...interface{}) {
	if globalLogger.shouldLog(ERROR) {
		log.Printf("[ERROR] "+msg, args...)
	}
}

// WARN - Situaciones problemáticas
func Warn(msg string, args ...interface{}) {
	if globalLogger.shouldLog(WARN) {
		log.Printf("[WARN] "+msg, args...)
	}
}

// INFO - Eventos importantes
func Info(msg string, args ...interface{}) {
	if globalLogger.shouldLog(INFO) {
		log.Printf("[INFO] "+msg, args...)
	}
}

// DEBUG - Detalles internos
func Debug(msg string, args ...interface{}) {
	if globalLogger.shouldLog(DEBUG) {
		log.Printf("[DEBUG] "+msg, args...)
	}
}

// Métricas de jobs - solo agregadas
func JobExecuted() {
	atomic.AddInt64(&globalLogger.jobMetrics.executed, 1)
	globalLogger.logMetricsIfNeeded()
}

func JobFailed() {
	atomic.AddInt64(&globalLogger.jobMetrics.failed, 1)
	globalLogger.logMetricsIfNeeded()
}

func (l *Logger) logMetricsIfNeeded() {
	if time.Since(l.lastMetrics) > 5*time.Minute {
		executed := atomic.SwapInt64(&l.jobMetrics.executed, 0)
		failed := atomic.SwapInt64(&l.jobMetrics.failed, 0)
		
		if executed > 0 || failed > 0 {
			Info("job_metrics executed=%d failed=%d", executed, failed)
		}
		
		l.lastMetrics = time.Now()
	}
}

// Logs específicos por categoría
func RaftError(msg string, args ...interface{}) {
	Error("raft: "+msg, args...)
}

func RaftWarn(msg string, args ...interface{}) {
	Warn("raft: "+msg, args...)
}

func RaftInfo(msg string, args ...interface{}) {
	Info("raft: "+msg, args...)
}

func ClusterError(msg string, args ...interface{}) {
	Error("cluster: "+msg, args...)
}

func ClusterWarn(msg string, args ...interface{}) {
	Warn("cluster: "+msg, args...)
}

func ClusterInfo(msg string, args ...interface{}) {
	Info("cluster: "+msg, args...)
}

func JobError(jobID, msg string, args ...interface{}) {
	Error("job %s: "+msg, append([]interface{}{jobID}, args...)...)
}

func JobWarn(jobID, msg string, args ...interface{}) {
	Warn("job %s: "+msg, append([]interface{}{jobID}, args...)...)
}
