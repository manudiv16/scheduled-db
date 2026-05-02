package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"scheduled-db/internal/logger"

	bolt "go.etcd.io/bbolt"
)

const coldSlotBucket = "cold_slots"

type BoltColdSlotStore struct {
	mu   sync.RWMutex
	db   *bolt.DB
	path string
}

func NewBoltColdSlotStore(dataDir string) (*BoltColdSlotStore, error) {
	dbPath := filepath.Join(dataDir, "cold_slots.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open cold slots database: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(coldSlotBucket))
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create cold slots bucket: %v", err)
	}

	logger.Debug("Opened cold slots store at %s", dbPath)
	return &BoltColdSlotStore{db: db, path: dbPath}, nil
}

func (b *BoltColdSlotStore) PutColdSlot(key int64, data *SlotData) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	serialized, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal cold slot %d: %v", key, err)
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(coldSlotBucket))
		if bucket == nil {
			return fmt.Errorf("cold slots bucket not found")
		}
		return bucket.Put(itob(key), serialized)
	})
}

func (b *BoltColdSlotStore) GetColdSlot(key int64) (*SlotData, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var data []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(coldSlotBucket))
		if bucket == nil {
			return fmt.Errorf("cold slots bucket not found")
		}
		data = bucket.Get(itob(key))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var slotData SlotData
	if err := json.Unmarshal(data, &slotData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cold slot %d: %v", key, err)
	}
	return &slotData, nil
}

func (b *BoltColdSlotStore) DeleteColdSlot(key int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(coldSlotBucket))
		if bucket == nil {
			return nil
		}
		return bucket.Delete(itob(key))
	})
}

func (b *BoltColdSlotStore) Close() error {
	return b.db.Close()
}

func (b *BoltColdSlotStore) Path() string {
	return b.path
}

func itob(v int64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}
