package slots

import (
	"sync"
	"time"

	"scheduled-db/internal/logger"
)

type WheelLevelConfig struct {
	Granularity time.Duration
	Buckets     int
}

func DefaultWheelConfigs(slotGap time.Duration) []WheelLevelConfig {
	if slotGap == 0 {
		slotGap = DefaultSlotGap
	}
	return []WheelLevelConfig{
		{Granularity: slotGap, Buckets: 360},
		{Granularity: slotGap * 360, Buckets: 24},
		{Granularity: slotGap * 360 * 24, Buckets: 365},
	}
}

type wheelLevel struct {
	granularity int64
	bucketCount int
	buckets     []map[int64]struct{}
	current     int
}

type slotLoc struct {
	level  int
	bucket int
}

type HierarchicalTimingWheel struct {
	mu           sync.RWMutex
	levels       []wheelLevel
	currentBase  int64
	slotGap      time.Duration
	slotLocation map[int64]slotLoc
	overflow     map[int64]struct{}
	initialized  bool
}

func NewHierarchicalTimingWheel(slotGap time.Duration, configs []WheelLevelConfig) *HierarchicalTimingWheel {
	if len(configs) == 0 {
		configs = DefaultWheelConfigs(slotGap)
	}

	levels := make([]wheelLevel, len(configs))
	for i, cfg := range configs {
		buckets := make([]map[int64]struct{}, cfg.Buckets)
		for j := range buckets {
			buckets[j] = make(map[int64]struct{})
		}
		levels[i] = wheelLevel{
			granularity: int64(cfg.Granularity.Seconds()),
			bucketCount: cfg.Buckets,
			buckets:     buckets,
			current:     0,
		}
	}

	return &HierarchicalTimingWheel{
		levels:       levels,
		slotGap:      slotGap,
		slotLocation: make(map[int64]slotLoc),
		overflow:     make(map[int64]struct{}),
		initialized:  false,
	}
}

func (htw *HierarchicalTimingWheel) Init(now int64) {
	htw.mu.Lock()
	defer htw.mu.Unlock()
	if htw.initialized {
		return
	}
	slotGapSeconds := int64(htw.slotGap.Seconds())
	if slotGapSeconds == 0 {
		slotGapSeconds = int64(DefaultSlotGap.Seconds())
	}
	htw.currentBase = (now / slotGapSeconds) * slotGapSeconds
	htw.initialized = true
	logger.Debug("HTW initialized with base=%d, slotGap=%ds", htw.currentBase, slotGapSeconds)
}

func (htw *HierarchicalTimingWheel) Add(slotKey int64) {
	htw.mu.Lock()
	defer htw.mu.Unlock()

	slotGapSeconds := int64(htw.slotGap.Seconds())
	if slotGapSeconds == 0 {
		slotGapSeconds = int64(DefaultSlotGap.Seconds())
	}
	slotTime := slotKey * slotGapSeconds

	if !htw.initialized {
		htw.currentBase = (slotTime / slotGapSeconds) * slotGapSeconds
		htw.initialized = true
	}

	delta := slotTime - htw.currentBase
	if delta < 0 {
		delta = 0
	}

	loc, ok := htw.findBucket(delta)
	if !ok {
		htw.overflow[slotKey] = struct{}{}
		return
	}

	htw.levels[loc.level].buckets[loc.bucket][slotKey] = struct{}{}
	htw.slotLocation[slotKey] = loc
}

func (htw *HierarchicalTimingWheel) Remove(slotKey int64) {
	htw.mu.Lock()
	defer htw.mu.Unlock()

	loc, ok := htw.slotLocation[slotKey]
	if ok {
		delete(htw.levels[loc.level].buckets[loc.bucket], slotKey)
		delete(htw.slotLocation, slotKey)
	} else {
		delete(htw.overflow, slotKey)
	}
}

func (htw *HierarchicalTimingWheel) Advance(now int64) []int64 {
	htw.mu.Lock()
	defer htw.mu.Unlock()

	if !htw.initialized {
		return nil
	}

	slotGapSeconds := int64(htw.slotGap.Seconds())
	if slotGapSeconds == 0 {
		slotGapSeconds = int64(DefaultSlotGap.Seconds())
	}

	newBase := (now / slotGapSeconds) * slotGapSeconds
	if newBase <= htw.currentBase {
		return nil
	}

	var ready []int64

	for htw.currentBase < newBase {
		htw.currentBase += slotGapSeconds

		bucket := htw.levels[0].buckets[htw.levels[0].current]
		for slotKey := range bucket {
			ready = append(ready, slotKey)
			delete(htw.slotLocation, slotKey)
		}
		htw.levels[0].buckets[htw.levels[0].current] = make(map[int64]struct{})
		htw.levels[0].current = (htw.levels[0].current + 1) % htw.levels[0].bucketCount

		if htw.levels[0].current == 0 {
			htw.cascade(1)
		}
	}

	overflowReady := make([]int64, 0)
	deltaMax := htw.totalCoverage()
	for slotKey := range htw.overflow {
		slotTime := slotKey * slotGapSeconds
		delta := slotTime - htw.currentBase
		if delta < 0 {
			delta = 0
		}
		if delta < deltaMax {
			loc, ok := htw.findBucket(delta)
			if ok {
				htw.levels[loc.level].buckets[loc.bucket][slotKey] = struct{}{}
				htw.slotLocation[slotKey] = loc
				overflowReady = append(overflowReady, slotKey)
			}
		}
	}
	for _, key := range overflowReady {
		delete(htw.overflow, key)
	}

	// Also promote from overflow if current time is close enough
	var promoteFromOverflow []int64
	for slotKey := range htw.overflow {
		slotTime := slotKey * slotGapSeconds
		if slotTime <= newBase {
			promoteFromOverflow = append(promoteFromOverflow, slotKey)
			ready = append(ready, slotKey)
		}
	}
	for _, key := range promoteFromOverflow {
		delete(htw.overflow, key)
		delete(htw.slotLocation, key)
	}

	return ready
}

func (htw *HierarchicalTimingWheel) GetReadySlots(now int64) []int64 {
	htw.mu.RLock()
	defer htw.mu.RUnlock()

	if !htw.initialized {
		return nil
	}

	slotGapSeconds := int64(htw.slotGap.Seconds())
	if slotGapSeconds == 0 {
		slotGapSeconds = int64(DefaultSlotGap.Seconds())
	}

	var ready []int64

	for _, level := range htw.levels {
		for bucketIdx := range level.buckets {
			for slotKey := range level.buckets[bucketIdx] {
				slotTime := slotKey * slotGapSeconds
				if slotTime <= now {
					ready = append(ready, slotKey)
				}
			}
		}
	}

	for slotKey := range htw.overflow {
		slotTime := slotKey * slotGapSeconds
		if slotTime <= now {
			ready = append(ready, slotKey)
		}
	}

	return ready
}

func (htw *HierarchicalTimingWheel) PeekMin() int64 {
	htw.mu.RLock()
	defer htw.mu.RUnlock()

	if !htw.initialized {
		return -1
	}

	slotGapSeconds := int64(htw.slotGap.Seconds())
	if slotGapSeconds == 0 {
		slotGapSeconds = int64(DefaultSlotGap.Seconds())
	}

	var minTime int64 = -1

	for i := range htw.levels {
		buckets := htw.levels[i].buckets
		count := htw.levels[i].bucketCount
		cur := htw.levels[i].current
		for offset := 0; offset < count; offset++ {
			idx := (cur + offset) % count
			if len(buckets[idx]) > 0 {
				for slotKey := range buckets[idx] {
					slotTime := slotKey * slotGapSeconds
					if minTime == -1 || slotTime < minTime {
						minTime = slotTime
					}
				}
				break
			}
		}
	}

	for slotKey := range htw.overflow {
		slotTime := slotKey * slotGapSeconds
		if minTime == -1 || slotTime < minTime {
			minTime = slotTime
		}
	}

	return minTime
}

func (htw *HierarchicalTimingWheel) cascade(level int) {
	if level >= len(htw.levels) {
		return
	}

	l := &htw.levels[level]
	bucket := l.buckets[l.current]
	l.buckets[l.current] = make(map[int64]struct{})

	for slotKey := range bucket {
		slotTime := htw.computeSlotTime(slotKey)
		delta := slotTime - htw.currentBase
		if delta < 0 {
			delta = 0
		}

		loc, ok := htw.findBucket(delta)
		if ok && loc.level < level {
			htw.levels[loc.level].buckets[loc.bucket][slotKey] = struct{}{}
			htw.slotLocation[slotKey] = loc
		} else {
			htw.overflow[slotKey] = struct{}{}
			delete(htw.slotLocation, slotKey)
		}
	}

	l.current = (l.current + 1) % l.bucketCount
	if l.current == 0 {
		htw.cascade(level + 1)
	}
}

func (htw *HierarchicalTimingWheel) findBucket(delta int64) (slotLoc, bool) {
	elapsed := int64(0)

	for i, level := range htw.levels {
		levelRange := level.granularity * int64(level.bucketCount)

		if delta < levelRange || i == len(htw.levels)-1 {
			bucketsIntoLevel := delta / level.granularity

			offset := bucketsIntoLevel - (elapsed / level.granularity)
			bucket := (level.current + int(offset)) % level.bucketCount

			bucket = max(0, min(bucket, level.bucketCount-1))

			return slotLoc{level: i, bucket: bucket}, true
		}

		elapsed = levelRange
	}

	return slotLoc{}, false
}

func (htw *HierarchicalTimingWheel) totalCoverage() int64 {
	total := int64(0)
	for _, level := range htw.levels {
		total += level.granularity * int64(level.bucketCount)
	}
	return total
}

func (htw *HierarchicalTimingWheel) computeSlotTime(slotKey int64) int64 {
	slotGapSeconds := int64(htw.slotGap.Seconds())
	if slotGapSeconds == 0 {
		slotGapSeconds = int64(DefaultSlotGap.Seconds())
	}
	return slotKey * slotGapSeconds
}

func (htw *HierarchicalTimingWheel) Size() int {
	htw.mu.RLock()
	defer htw.mu.RUnlock()

	count := 0
	for _, level := range htw.levels {
		for _, bucket := range level.buckets {
			count += len(bucket)
		}
	}
	count += len(htw.overflow)
	return count
}

func (htw *HierarchicalTimingWheel) Reset() {
	htw.mu.Lock()
	defer htw.mu.Unlock()

	for i := range htw.levels {
		for j := range htw.levels[i].buckets {
			htw.levels[i].buckets[j] = make(map[int64]struct{})
		}
		htw.levels[i].current = 0
	}
	htw.slotLocation = make(map[int64]slotLoc)
	htw.overflow = make(map[int64]struct{})
	htw.initialized = false
}
