package simulator

import (
	"sync"
	"time"
)

type SimulatedClock struct {
	mu    sync.RWMutex
	now   int64
	speed float64
}

func NewSimulatedClock(startAt int64) *SimulatedClock {
	if startAt == 0 {
		startAt = time.Now().Unix()
	}
	return &SimulatedClock{
		now:   startAt,
		speed: 1.0,
	}
}

func (c *SimulatedClock) Now() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *SimulatedClock) SetSpeed(speed float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if speed < 0 {
		speed = 0
	}
	c.speed = speed
}

func (c *SimulatedClock) GetSpeed() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.speed
}

func (c *SimulatedClock) SetTime(t int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func (c *SimulatedClock) Advance(seconds float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now += int64(seconds * c.speed)
}

func (c *SimulatedClock) Tick(elapsed float64) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now += int64(elapsed * c.speed)
	return c.now
}
