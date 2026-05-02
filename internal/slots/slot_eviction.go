package slots

import (
	"sync"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/store"
)

const (
	DefaultHotWindow     = 48 * time.Hour
	DefaultCheckInterval = 5 * time.Minute
)

type SlotEvictionConfig struct {
	Enabled       bool
	HotWindow     time.Duration
	CheckInterval time.Duration
}

func DefaultSlotEvictionConfig() SlotEvictionConfig {
	return SlotEvictionConfig{
		Enabled:       false,
		HotWindow:     DefaultHotWindow,
		CheckInterval: DefaultCheckInterval,
	}
}

type SlotEvictor struct {
	mu      sync.Mutex
	config  SlotEvictionConfig
	store   *store.Store
	stopCh  chan struct{}
	running bool
}

func NewSlotEvictor(store *store.Store, config SlotEvictionConfig) *SlotEvictor {
	if config.HotWindow == 0 {
		config.HotWindow = DefaultHotWindow
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = DefaultCheckInterval
	}
	return &SlotEvictor{
		config: config,
		store:  store,
		stopCh: make(chan struct{}),
	}
}

func (se *SlotEvictor) Start() {
	se.mu.Lock()
	if se.running {
		se.mu.Unlock()
		return
	}
	se.running = true
	se.mu.Unlock()

	go se.run()
	logger.Info("slot evictor started: hot_window=%s, check_interval=%s", se.config.HotWindow, se.config.CheckInterval)
}

func (se *SlotEvictor) Stop() {
	se.mu.Lock()
	defer se.mu.Unlock()
	if !se.running {
		return
	}
	close(se.stopCh)
	se.running = false
	logger.Info("slot evictor stopped")
}

func (se *SlotEvictor) run() {
	ticker := time.NewTicker(se.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-se.stopCh:
			return
		case <-ticker.C:
			se.evict()
		}
	}
}

func (se *SlotEvictor) evict() {
	if !se.store.IsLeader() {
		return
	}

	now := time.Now().Unix()
	hotWindowSeconds := int64(se.config.HotWindow.Seconds())
	hotThreshold := now - hotWindowSeconds

	hotSlots := se.store.GetAllSlots()
	evicted := 0
	for key, slotData := range hotSlots {
		if slotData.MaxTime < hotThreshold {
			if err := se.store.ArchiveSlot(key); err != nil {
				logger.Debug("failed to archive slot %d: %v", key, err)
				continue
			}
			evicted++
		}
	}

	coldKeys := se.store.GetColdSlotKeys()
	promoted := 0
	for _, key := range coldKeys {
		slotData, exists := se.store.GetSlot(key)
		if !exists {
			continue
		}
		if slotData.MinTime >= hotThreshold && slotData.MaxTime < hotThreshold+hotWindowSeconds*2 {
			if err := se.store.UnarchiveSlot(key); err != nil {
				logger.Debug("failed to unarchive slot %d: %v", key, err)
				continue
			}
			promoted++
		}
	}

	if evicted > 0 || promoted > 0 {
		logger.Info("slot eviction: evicted=%d, promoted=%d, hot=%d, cold=%d",
			evicted, promoted, se.store.GetHotSlotCount(), se.store.GetColdSlotCount())
	}
}
