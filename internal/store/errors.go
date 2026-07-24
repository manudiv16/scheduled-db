//go:build !wasm

package store

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure modes.
var (
	ErrNotLeader           = errors.New("not leader")
	ErrNotFound            = errors.New("not found")
	ErrColdStoreNotEnabled = errors.New("cold store not enabled")
	ErrJobNotFound         = errors.New("job not found")
	ErrSlotNotFound        = errors.New("slot not found")
	ErrInvalidCommand      = errors.New("invalid command")
)

// NotLeaderError wraps ErrNotLeader with optional context.
func NotLeaderError(context string) error {
	if context == "" {
		return ErrNotLeader
	}
	return fmt.Errorf("%s: %w", context, ErrNotLeader)
}

// NotFoundError wraps ErrNotFound with the entity name.
func NotFoundError(entity string) error {
	if entity == "" {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", entity, ErrNotFound)
}
