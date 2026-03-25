package storage

import (
	"os"
	"sync"
	"time"
)

// Storage tracks temp files and schedules deletion.
type Storage struct {
	ttl time.Duration
	mu  sync.Mutex
}

// New creates a storage helper with the given time-to-live for files.
func New(ttl time.Duration) *Storage {
	return &Storage{ttl: ttl}
}

// ScheduleDelete removes paths after TTL (best-effort).
func (s *Storage) ScheduleDelete(paths ...string) {
	if s == nil || s.ttl <= 0 {
		return
	}
	delay := s.ttl
	time.AfterFunc(delay, func() {
		for _, p := range paths {
			_ = os.Remove(p)
		}
	})
}
