package slots

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"scheduled-db/internal/store"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

type mockColdStore struct {
	slots map[int64]*store.SlotData
}

func newMockColdStore() *mockColdStore {
	return &mockColdStore{slots: make(map[int64]*store.SlotData)}
}

func (m *mockColdStore) PutColdSlot(key int64, data *store.SlotData) error {
	m.slots[key] = data
	return nil
}

func (m *mockColdStore) GetColdSlot(key int64) (*store.SlotData, error) {
	return m.slots[key], nil
}

func (m *mockColdStore) DeleteColdSlot(key int64) error {
	delete(m.slots, key)
	return nil
}

func (m *mockColdStore) Close() error { return nil }

func TestSlotEvictionConfig_Defaults(t *testing.T) {
	cfg := DefaultSlotEvictionConfig()
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 48*time.Hour, cfg.HotWindow)
	assert.Equal(t, 5*time.Minute, cfg.CheckInterval)
}

func TestSlotEvictor_ZeroDefaults(t *testing.T) {
	cfg := SlotEvictionConfig{Enabled: true}
	evictor := NewSlotEvictor(nil, cfg)
	assert.Equal(t, DefaultHotWindow, evictor.config.HotWindow)
	assert.Equal(t, DefaultCheckInterval, evictor.config.CheckInterval)
}

func TestSlotEvictor_StartStop(t *testing.T) {
	evictor := NewSlotEvictor(nil, SlotEvictionConfig{
		Enabled:       true,
		HotWindow:     1 * time.Hour,
		CheckInterval: 1 * time.Second,
	})

	evictor.Start()
	assert.True(t, evictor.running)

	evictor.Stop()
	assert.False(t, evictor.running)
}

func TestSlotEvictor_DoubleStartStop(t *testing.T) {
	evictor := NewSlotEvictor(nil, SlotEvictionConfig{
		Enabled:       true,
		HotWindow:     1 * time.Hour,
		CheckInterval: 1 * time.Second,
	})

	evictor.Start()
	evictor.Start()

	evictor.Stop()
	evictor.Stop()
}

func TestFSMArchiveUnarchiveIntegration(t *testing.T) {
	cs := newMockColdStore()
	fsm := store.NewFSMWithColdStore(cs)

	now := time.Now().Unix()
	pastKey := int64((now - 100000) / 10)
	futureKey := int64((now + 100000) / 10)

	pastSlot := &store.SlotData{Key: pastKey, MinTime: pastKey * 10, MaxTime: pastKey*10 + 9, JobIDs: []string{"j1"}}
	futureSlot := &store.SlotData{Key: futureKey, MinTime: futureKey * 10, MaxTime: futureKey*10 + 9, JobIDs: []string{"j2"}}

	fsm.Apply(&raft.Log{Data: mustMarshalCommand(store.Command{Type: store.CommandCreateSlot, Slot: pastSlot})})
	fsm.Apply(&raft.Log{Data: mustMarshalCommand(store.Command{Type: store.CommandCreateSlot, Slot: futureSlot})})

	assert.Equal(t, 2, fsm.GetHotSlotCount())

	fsm.Apply(&raft.Log{Data: mustMarshalCommand(store.Command{Type: store.CommandArchiveSlot, ColdSlot: pastSlot})})

	assert.Equal(t, 1, fsm.GetHotSlotCount())
	assert.Equal(t, 1, fsm.GetColdSlotCount())
	assert.True(t, fsm.IsSlotCold(pastKey))
	assert.False(t, fsm.IsSlotCold(futureKey))

	got, exists := fsm.GetSlot(pastKey)
	assert.True(t, exists)
	assert.Equal(t, pastKey, got.Key)

	fsm.Apply(&raft.Log{Data: mustMarshalCommand(store.Command{Type: store.CommandUnarchiveSlot, ID: formatKey(pastKey)})})

	assert.Equal(t, 2, fsm.GetHotSlotCount())
	assert.Equal(t, 0, fsm.GetColdSlotCount())
	assert.False(t, fsm.IsSlotCold(pastKey))
}

func TestFSMArchiveEmptySlotDeletes(t *testing.T) {
	cs := newMockColdStore()
	fsm := store.NewFSMWithColdStore(cs)

	slot := &store.SlotData{Key: 50, MinTime: 500, MaxTime: 509, JobIDs: []string{}}
	fsm.Apply(&raft.Log{Data: mustMarshalCommand(store.Command{Type: store.CommandArchiveSlot, ColdSlot: slot})})

	assert.Equal(t, 1, fsm.GetColdSlotCount())

	fsm.Apply(&raft.Log{Data: mustMarshalCommand(store.Command{Type: store.CommandDeleteSlot, ID: "50"})})

	assert.Equal(t, 0, fsm.GetColdSlotCount())
	assert.False(t, fsm.IsSlotCold(50))
	_, exists := fsm.GetSlot(50)
	assert.False(t, exists)
}

func mustMarshalCommand(cmd store.Command) []byte {
	data, err := json.Marshal(cmd)
	if err != nil {
		panic(err)
	}
	return data
}

func formatKey(key int64) string {
	return fmt.Sprintf("%d", key)
}
