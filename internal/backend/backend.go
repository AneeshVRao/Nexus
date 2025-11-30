package backend

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// Backend represents a backend server
type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

// SetAlive sets the alive status of the backend in a thread-safe manner
func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	defer b.mux.Unlock()
	b.Alive = alive
}

// IsAlive returns whether the backend is alive in a thread-safe manner
func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.Alive
}

// passiveHealthCheckTransport wraps http.RoundTripper to detect connection failures
type passiveHealthCheckTransport struct {
	backend   *Backend
	transport http.RoundTripper
}

func (t *passiveHealthCheckTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.transport.RoundTrip(req)

	if err != nil {
		// Connection error detected - mark backend as down immediately
		if t.backend.IsAlive() {
			log.Printf("[PASSIVE] Backend %s failed: %v - marking as DOWN", t.backend.URL.String(), err)
			t.backend.SetAlive(false)
		}
		return nil, err
	}

	// Check for 5xx errors which might indicate backend issues
	if resp.StatusCode >= 500 {
		log.Printf("[PASSIVE] Backend %s returned %d - marking as DOWN", t.backend.URL.String(), resp.StatusCode)
		t.backend.SetAlive(false)
	}

	return resp, nil
}

// NewBackend creates a new Backend instance from a URL string
func NewBackend(urlStr string) (*Backend, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	backend := &Backend{
		URL:          parsedURL,
		Alive:        true,
		ReverseProxy: httputil.NewSingleHostReverseProxy(parsedURL),
	}

	// Create custom transport with passive health checking
	defaultTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	// Wrap the transport with passive health checking
	backend.ReverseProxy.Transport = &passiveHealthCheckTransport{
		backend:   backend,
		transport: defaultTransport,
	}

	log.Printf("Created backend %s with passive health check enabled", parsedURL.String())
	return backend, nil
}
