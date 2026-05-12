package slots

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultWheelConfigs(t *testing.T) {
	configs := DefaultWheelConfigs(10 * time.Second)
	assert.Len(t, configs, 3)
	assert.Equal(t, 10*time.Second, configs[0].Granularity)
	assert.Equal(t, 3600*time.Second, configs[1].Granularity)
	assert.Equal(t, 86400*time.Second, configs[2].Granularity)
	assert.Equal(t, 360, configs[0].Buckets)
	assert.Equal(t, 24, configs[1].Buckets)
	assert.Equal(t, 365, configs[2].Buckets)
}

func TestNewHierarchicalTimingWheel(t *testing.T) {
	configs := DefaultWheelConfigs(10 * time.Second)
	wheel := NewHierarchicalTimingWheel(10*time.Second, configs)
	assert.NotNil(t, wheel)
	assert.Equal(t, 3, len(wheel.levels))
	assert.False(t, wheel.initialized)
}

func TestTimingWheel_AddAndGet(t *testing.T) {
	slotGap := 10 * time.Second
	wheel := NewHierarchicalTimingWheel(slotGap, nil)

	now := time.Now().Unix()
	wheel.Init(now)

	slotGapSec := int64(slotGap.Seconds())
	slotKey := now/slotGapSec + 5

	wheel.Add(slotKey)
	assert.Equal(t, 1, wheel.Size())

	ready := wheel.Advance(now + slotGapSec*6)
	assert.Contains(t, ready, slotKey)
	assert.Equal(t, 0, wheel.Size())
}

func TestTimingWheel_AddPast(t *testing.T) {
	slotGap := 10 * time.Second
	wheel := NewHierarchicalTimingWheel(slotGap, nil)

	now := time.Now().Unix()
	wheel.Init(now)

	slotGapSec := int64(slotGap.Seconds())
	pastSlot := now/slotGapSec - 5

	wheel.Add(pastSlot)

	ready := wheel.Advance(now + slotGapSec)
	assert.Contains(t, ready, pastSlot)
}

func TestTimingWheel_Remove(t *testing.T) {
	slotGap := 10 * time.Second
	wheel := NewHierarchicalTimingWheel(slotGap, nil)

	now := time.Now().Unix()
	wheel.Init(now)

	slotGapSec := int64(slotGap.Seconds())
	slotKey := now/slotGapSec + 10

	wheel.Add(slotKey)
	assert.Equal(t, 1, wheel.Size())

	wheel.Remove(slotKey)
	assert.Equal(t, 0, wheel.Size())
}

func TestTimingWheel_MultipleSlots(t *testing.T) {
	slotGap := 10 * time.Second
	wheel := NewHierarchicalTimingWheel(slotGap, nil)

	now := time.Now().Unix()
	slotGapSec := int64(slotGap.Seconds())

	baseKey := now / slotGapSec
	wheel.Init(now)

	for i := int64(0); i < 5; i++ {
		wheel.Add(baseKey + i)
	}
	assert.Equal(t, 5, wheel.Size())

	var allReady []int64
	for i := 0; i < 10; i++ {
		ready := wheel.Advance(now + slotGapSec*int64(i+1))
		for _, k := range ready {
			allReady = append(allReady, k)
		}
	}
	assert.Len(t, allReady, 5)
}

func TestTimingWheel_Overflow(t *testing.T) {
	slotGap := 10 * time.Second
	configs := []WheelLevelConfig{
		{Granularity: slotGap, Buckets: 6},
		{Granularity: slotGap * 6, Buckets: 6},
		{Granularity: slotGap * 36, Buckets: 6},
	}
	wheel := NewHierarchicalTimingWheel(slotGap, configs)

	now := time.Now().Unix()
	slotGapSec := int64(slotGap.Seconds())
	wheel.Init(now)

	farFutureSlot := now/slotGapSec + 1000
	wheel.Add(farFutureSlot)
	assert.Equal(t, 1, wheel.Size())

	ready := wheel.Advance(now + slotGapSec)
	assert.Empty(t, ready)
	assert.Equal(t, 1, wheel.Size())
}

func TestTimingWheel_Reset(t *testing.T) {
	slotGap := 10 * time.Second
	wheel := NewHierarchicalTimingWheel(slotGap, nil)

	now := time.Now().Unix()
	slotGapSec := int64(slotGap.Seconds())
	wheel.Init(now)

	wheel.Add(now/slotGapSec + 5)
	wheel.Add(now/slotGapSec + 10)
	assert.Equal(t, 2, wheel.Size())

	wheel.Reset()
	assert.Equal(t, 0, wheel.Size())
	assert.False(t, wheel.initialized)
}

func TestTimingWheel_Cascade(t *testing.T) {
	slotGap := 10 * time.Second
	configs := []WheelLevelConfig{
		{Granularity: slotGap, Buckets: 6},
		{Granularity: slotGap * 6, Buckets: 6},
		{Granularity: slotGap * 36, Buckets: 6},
	}
	wheel := NewHierarchicalTimingWheel(slotGap, configs)

	now := time.Now().Unix()
	slotGapSec := int64(slotGap.Seconds())
	wheel.Init(now)

	nearSlot := now/slotGapSec + 3
	wheel.Add(nearSlot)

	midSlot := now/slotGapSec + 10
	wheel.Add(midSlot)

	assert.Equal(t, 2, wheel.Size())

	ready := wheel.Advance(now + slotGapSec*4)
	assert.Contains(t, ready, nearSlot)

	ready = wheel.Advance(now + slotGapSec*12)
	assert.Contains(t, ready, midSlot)
}

func TestTimingWheel_PeekMin(t *testing.T) {
	slotGap := 10 * time.Second
	wheel := NewHierarchicalTimingWheel(slotGap, nil)

	now := time.Now().Unix()
	slotGapSec := int64(slotGap.Seconds())
	wheel.Init(now)

	minTime := wheel.PeekMin()
	assert.Equal(t, int64(-1), minTime)

	slot1 := now/slotGapSec + 5
	slot2 := now/slotGapSec + 2
	wheel.Add(slot1)
	wheel.Add(slot2)

	minTime = wheel.PeekMin()
	expectedMin := slot2 * slotGapSec
	assert.Equal(t, expectedMin, minTime)
}

func TestTimingWheel_InitIdempotent(t *testing.T) {
	slotGap := 10 * time.Second
	wheel := NewHierarchicalTimingWheel(slotGap, nil)

	now := time.Now().Unix()
	wheel.Init(now)
	wheel.Init(now + 1000)

	assert.True(t, wheel.initialized)
}
