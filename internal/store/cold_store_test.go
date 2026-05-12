package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltColdSlotStore_PutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs.Close()

	slot := &SlotData{
		Key:     100,
		MinTime: 1000,
		MaxTime: 1099,
		JobIDs:  []string{"job-1", "job-2"},
	}

	err = cs.PutColdSlot(100, slot)
	require.NoError(t, err)

	got, err := cs.GetColdSlot(100)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, slot.Key, got.Key)
	assert.Equal(t, slot.MinTime, got.MinTime)
	assert.Equal(t, slot.MaxTime, got.MaxTime)
	assert.Equal(t, slot.JobIDs, got.JobIDs)
}

func TestBoltColdSlotStore_GetMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs.Close()

	got, err := cs.GetColdSlot(999)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestBoltColdSlotStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs.Close()

	slot := &SlotData{Key: 50, MinTime: 500, MaxTime: 599, JobIDs: []string{"j1"}}
	err = cs.PutColdSlot(50, slot)
	require.NoError(t, err)

	err = cs.DeleteColdSlot(50)
	require.NoError(t, err)

	got, err := cs.GetColdSlot(50)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestFSMWithColdStore_ArchiveSlot(t *testing.T) {
	tmpDir := t.TempDir()
	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs.Close()

	fsm := NewFSMWithColdStore(cs)

	slot := &SlotData{Key: 10, MinTime: 100, MaxTime: 199, JobIDs: []string{"j1"}}
	cmd := Command{Type: CommandCreateSlot, Slot: slot}
	data, _ := json.Marshal(cmd)
	fsm.Apply(&raft.Log{Data: data})

	assert.True(t, fsm.IsSlotCold(10) == false)
	assert.Equal(t, 1, fsm.GetHotSlotCount())

	archiveCmd := Command{Type: CommandArchiveSlot, ColdSlot: slot}
	data, _ = json.Marshal(archiveCmd)
	fsm.Apply(&raft.Log{Data: data})

	assert.True(t, fsm.IsSlotCold(10))
	assert.Equal(t, 0, fsm.GetHotSlotCount())
	assert.Equal(t, 1, fsm.GetColdSlotCount())

	got, exists := fsm.GetSlot(10)
	assert.True(t, exists)
	assert.Equal(t, int64(10), got.Key)
}

func TestFSMWithColdStore_UnarchiveSlot(t *testing.T) {
	tmpDir := t.TempDir()
	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs.Close()

	fsm := NewFSMWithColdStore(cs)

	slot := &SlotData{Key: 20, MinTime: 200, MaxTime: 299, JobIDs: []string{"j1"}}
	archiveCmd := Command{Type: CommandArchiveSlot, ColdSlot: slot}
	data, _ := json.Marshal(archiveCmd)
	fsm.Apply(&raft.Log{Data: data})

	assert.True(t, fsm.IsSlotCold(20))
	assert.Equal(t, 1, fsm.GetColdSlotCount())

	unarchiveCmd := Command{Type: CommandUnarchiveSlot, ID: "20"}
	data, _ = json.Marshal(unarchiveCmd)
	fsm.Apply(&raft.Log{Data: data})

	assert.False(t, fsm.IsSlotCold(20))
	assert.Equal(t, 1, fsm.GetHotSlotCount())
	assert.Equal(t, 0, fsm.GetColdSlotCount())

	got, exists := fsm.GetSlot(20)
	assert.True(t, exists)
	assert.Equal(t, int64(20), got.Key)
}

func TestFSMWithColdStore_DeleteSlotCleansCold(t *testing.T) {
	tmpDir := t.TempDir()
	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs.Close()

	fsm := NewFSMWithColdStore(cs)

	slot := &SlotData{Key: 30, MinTime: 300, MaxTime: 399, JobIDs: []string{"j1"}}
	archiveCmd := Command{Type: CommandArchiveSlot, ColdSlot: slot}
	data, _ := json.Marshal(archiveCmd)
	fsm.Apply(&raft.Log{Data: data})

	assert.True(t, fsm.IsSlotCold(30))

	deleteCmd := Command{Type: CommandDeleteSlot, ID: "30"}
	data, _ = json.Marshal(deleteCmd)
	fsm.Apply(&raft.Log{Data: data})

	assert.False(t, fsm.IsSlotCold(30))
	_, exists := fsm.GetSlot(30)
	assert.False(t, exists)
}

func TestFSMWithColdStore_SnapshotRestore(t *testing.T) {
	tmpDir := t.TempDir()
	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs.Close()

	fsm := NewFSMWithColdStore(cs)

	hotSlot := &SlotData{Key: 1, MinTime: 10, MaxTime: 19, JobIDs: []string{"j1"}}
	cmd := Command{Type: CommandCreateSlot, Slot: hotSlot}
	data, _ := json.Marshal(cmd)
	fsm.Apply(&raft.Log{Data: data})

	coldSlot := &SlotData{Key: 100, MinTime: 1000, MaxTime: 1099, JobIDs: []string{"j2"}}
	archiveCmd := Command{Type: CommandArchiveSlot, ColdSlot: coldSlot}
	data, _ = json.Marshal(archiveCmd)
	fsm.Apply(&raft.Log{Data: data})

	snap, err := fsm.Snapshot()
	require.NoError(t, err)

	var buf bytes.Buffer
	sink := &testSink{writer: &buf}
	err = snap.Persist(sink)
	require.NoError(t, err)

	fsm2 := NewFSMWithColdStore(cs)
	err = fsm2.Restore(&testReadCloser{data: buf.Bytes()})
	require.NoError(t, err)

	assert.Equal(t, 1, fsm2.GetHotSlotCount())
	assert.Equal(t, 1, fsm2.GetColdSlotCount())
	assert.True(t, fsm2.IsSlotCold(100))
	assert.False(t, fsm2.IsSlotCold(1))
}

func TestFSMWithoutColdStore_ArchiveSlotFails(t *testing.T) {
	fsm := NewFSM()

	slot := &SlotData{Key: 10, MinTime: 100, MaxTime: 199, JobIDs: []string{"j1"}}
	cmd := Command{Type: CommandCreateSlot, Slot: slot}
	data, _ := json.Marshal(cmd)
	fsm.Apply(&raft.Log{Data: data})

	archiveCmd := Command{Type: CommandArchiveSlot, ColdSlot: slot}
	data, _ = json.Marshal(archiveCmd)
	result := fsm.Apply(&raft.Log{Data: data})

	err, ok := result.(error)
	assert.True(t, ok)
	assert.Error(t, err)
}

type testSink struct {
	writer *bytes.Buffer
	closed bool
}

func (ts *testSink) Write(p []byte) (int, error) { return ts.writer.Write(p) }
func (ts *testSink) Close() error                { ts.closed = true; return nil }
func (ts *testSink) ID() string                  { return "test" }
func (ts *testSink) Cancel() error               { return nil }

type testReadCloser struct {
	data   []byte
	offset int
}

func (trc *testReadCloser) Read(p []byte) (int, error) {
	if trc.offset >= len(trc.data) {
		return 0, os.ErrClosed
	}
	n := copy(p, trc.data[trc.offset:])
	trc.offset += n
	return n, nil
}

func (trc *testReadCloser) Close() error { return nil }

func TestColdStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cold_slots.db")

	cs, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)

	slot := &SlotData{Key: 42, MinTime: 420, MaxTime: 499, JobIDs: []string{"j1", "j2", "j3"}}
	err = cs.PutColdSlot(42, slot)
	require.NoError(t, err)
	cs.Close()

	cs2, err := NewBoltColdSlotStore(tmpDir)
	require.NoError(t, err)
	defer cs2.Close()

	assert.Equal(t, dbPath, cs2.Path())

	got, err := cs2.GetColdSlot(42)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(42), got.Key)
	assert.Equal(t, []string{"j1", "j2", "j3"}, got.JobIDs)
}
