package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nexus-lb/nexus/internal/backend"
	"github.com/nexus-lb/nexus/internal/health"
	"github.com/nexus-lb/nexus/internal/pool"
)

const (
	proxyPort       = ":8000"
	healthInterval  = 10 * time.Second
	healthTimeout   = 2 * time.Second
	shutdownTimeout = 30 * time.Second
	maxRetries      = 3
)

var (
	backendURLs = []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}
)

func main() {
	// Create the server pool
	serverPool := &pool.ServerPool{}

	// Add backends to the pool
	for _, urlStr := range backendURLs {
		backend, err := backend.NewBackend(urlStr)
		if err != nil {
			log.Fatalf("Failed to create backend for %s: %v", urlStr, err)
		}
		serverPool.AddBackend(backend)
		log.Printf("Added backend: %s", urlStr)
	}

	// Log startup information
	log.Printf("Nexus load balancer starting on port %s", proxyPort)
	log.Printf("Load balancing across %d backends", serverPool.GetPoolSize())

	// Create and start health checker
	healthChecker := health.NewHealthChecker(serverPool, healthInterval, healthTimeout)
	healthChecker.Start()

	// Create HTTP server with load balancing handler
	server := &http.Server{
		Addr: proxyPort,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// Try up to maxRetries times to find a working backend
			attempts := 0

			for attempts < maxRetries {
				attempts++

				// Get the next available peer
				peer := serverPool.GetNextPeer()
				if peer == nil {
					if attempts < maxRetries {
						time.Sleep(10 * time.Millisecond) // Brief pause before retry
						continue
					}
					log.Printf("[%s] %s %s -> NO BACKEND AVAILABLE (503)",
						startTime.Format("2006-01-02 15:04:05"),
						r.Method,
						r.URL.Path)
					http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
					return
				}

				// Check if backend is alive before proxying
				if !peer.IsAlive() {
					log.Printf("[%s] %s %s -> %s is marked DOWN, trying next (attempt %d)",
						startTime.Format("2006-01-02 15:04:05"),
						r.Method,
						r.URL.Path,
						peer.URL.String(),
						attempts)
					continue
				}

				// Log the request with backend information
				log.Printf("[%s] %s %s -> %s (attempt %d)",
					startTime.Format("2006-01-02 15:04:05"),
					r.Method,
					r.URL.Path,
					peer.URL.String(),
					attempts)

				// Add custom headers
				w.Header().Set("X-Forwarded-By", "Nexus")
				w.Header().Set("X-Backend-Server", peer.URL.String())

				// Forward the request to the selected backend
				// The custom transport will mark backend as DOWN if it fails
				peer.ReverseProxy.ServeHTTP(w, r)
				return
			}

			// If we get here, all retries failed
			log.Printf("[%s] %s %s -> ALL RETRIES FAILED (503)",
				startTime.Format("2006-01-02 15:04:05"),
				r.Method,
				r.URL.Path)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		}),
	}

	log.Printf("Nexus is ready to accept connections")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Println("\nReceived shutdown signal, gracefully shutting down...")

	// Stop health checker
	healthChecker.Stop()

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Nexus shut down successfully")
}
