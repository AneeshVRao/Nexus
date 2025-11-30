package health

import (
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/nexus-lb/nexus/internal/pool"
)

// HealthChecker performs periodic health checks on backend servers
type HealthChecker struct {
	pool     *pool.ServerPool
	interval time.Duration
	timeout  time.Duration
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewHealthChecker creates a new health checker instance
func NewHealthChecker(pool *pool.ServerPool, interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		pool:     pool,
		interval: interval,
		timeout:  timeout,
		stopChan: make(chan struct{}),
	}
}

// Start launches the health checker in a separate goroutine
func (h *HealthChecker) Start() {
	log.Printf("Health checker starting (interval: %v, timeout: %v)", h.interval, h.timeout)

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		// Run initial health check immediately
		h.checkHealth()

		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.checkHealth()
			case <-h.stopChan:
				log.Println("Health checker stopped")
				return
			}
		}
	}()
}

// Stop gracefully stops the health checker
func (h *HealthChecker) Stop() {
	log.Println("Stopping health checker...")
	close(h.stopChan)
	h.wg.Wait()
}

// checkHealth iterates through all backends and tests their health
func (h *HealthChecker) checkHealth() {
	backends := h.pool.GetBackends()

	for _, backend := range backends {
		alive := h.isBackendAlive(backend.URL)
		wasAlive := backend.IsAlive()

		if alive != wasAlive {
			if alive {
				log.Printf("Backend %s recovered (DOWN -> UP)", backend.URL.String())
			} else {
				log.Printf("Backend %s failed health check (UP -> DOWN)", backend.URL.String())
			}
			backend.SetAlive(alive)
		}
	}
}

// isBackendAlive checks if a backend is reachable by attempting a TCP connection
func (h *HealthChecker) isBackendAlive(u *url.URL) bool {
	// Extract host and port from URL
	host := u.Host

	// If no port is specified, use default HTTP port
	if u.Port() == "" {
		if u.Scheme == "https" {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}

	// Attempt TCP connection with timeout
	conn, err := net.DialTimeout("tcp", host, h.timeout)
	if err != nil {
		return false
	}

	// Connection successful, close it and return true
	conn.Close()
	return true
}
