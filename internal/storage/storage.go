package storage

import (
	"os"
	"sync"
	"time"
)

type Storage struct {
	ttl time.Duration
	mu  sync.Mutex
	wg  sync.WaitGroup
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
	s.wg.Add(1)
	time.AfterFunc(delay, func() {
		for _, p := range paths {
			_ = os.Remove(p)
		}
		s.wg.Done()
	})
}

// Shutdown waits for all pending deletions to complete, up to the given timeout.
// If the timeout is reached, it returns without further blocking.
func (s *Storage) Shutdown(timeout time.Duration) {
	if s == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}
