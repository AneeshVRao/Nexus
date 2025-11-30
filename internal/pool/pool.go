package pool

import (
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/nexus-lb/nexus/internal/backend"
)

// ServerPool represents a pool of backend servers
type ServerPool struct {
	backends []*backend.Backend
	current  uint64
	mux      sync.RWMutex
}

// AddBackend adds a backend to the server pool
func (s *ServerPool) AddBackend(b *backend.Backend) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.backends = append(s.backends, b)
}

// GetPoolSize returns the number of backends in the pool safely
func (s *ServerPool) GetPoolSize() int {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return len(s.backends)
}

// NextIndex atomically increments the counter and returns the next index
func (s *ServerPool) NextIndex() int {
	s.mux.RLock()
	poolSize := len(s.backends)
	s.mux.RUnlock()

	if poolSize == 0 {
		return 0
	}

	return int(atomic.AddUint64(&s.current, 1) % uint64(poolSize))
}

// GetNextPeer returns the next alive backend using round-robin selection
func (s *ServerPool) GetNextPeer() *backend.Backend {
	s.mux.RLock()
	poolSize := len(s.backends)
	s.mux.RUnlock()

	if poolSize == 0 {
		return nil
	}

	// Start from the next index
	next := s.NextIndex()

	// Try to find an alive backend, starting from next and wrapping around
	for i := 0; i < poolSize; i++ {
		idx := (next + i) % poolSize

		s.mux.RLock()
		backend := s.backends[idx]
		s.mux.RUnlock()

		if backend.IsAlive() {
			// Update current index to the selected backend
			atomic.StoreUint64(&s.current, uint64(idx))
			return backend
		}
	}

	// No alive backends found
	return nil
}

// MarkBackendStatus updates the alive status of a backend by URL
func (s *ServerPool) MarkBackendStatus(backendURL *url.URL, alive bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	for _, b := range s.backends {
		if b.URL.String() == backendURL.String() {
			b.SetAlive(alive)
			break
		}
	}
}

// GetBackends returns a copy of the backends slice for health checking
func (s *ServerPool) GetBackends() []*backend.Backend {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// Return a copy to avoid race conditions
	backends := make([]*backend.Backend, len(s.backends))
	copy(backends, s.backends)
	return backends
}

// GetPoolStatus returns the current pool status (alive/total backends)
func (s *ServerPool) GetPoolStatus() (alive, total int) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	total = len(s.backends)
	for _, b := range s.backends {
		if b.IsAlive() {
			alive++
		}
	}
	return alive, total
}
