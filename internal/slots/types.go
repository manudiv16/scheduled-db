package slots

import (
	"scheduled-db/internal/store"
	"time"
)

const DefaultSlotGap = 10 * time.Second

type SlotKey int64

type Slot struct {
	Key     SlotKey
	MinTime int64
	MaxTime int64
	Jobs    []*store.Job
}
