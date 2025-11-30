# Nexus Load Balancer

A production-grade HTTP load balancer written in Go with intelligent health checking, automatic failover, and round-robin distribution.

## Features

### Phase 1: Basic Reverse Proxy ✅
- Single-server HTTP reverse proxy
- Request forwarding with proper header handling
- Clean project structure

### Phase 2: Round-Robin Load Balancing ✅
- Multi-backend server pool
- Round-robin request distribution
- Thread-safe backend management
- Atomic counter for consistent distribution

### Phase 3: Thread Safety & Observability ✅
- Thread-safe operations with RWMutex
- Detailed request logging with timestamps
- Custom response headers:
  - `X-Forwarded-By: Nexus`
  - `X-Backend-Server: <backend-url>`
- Graceful shutdown handling (SIGINT/SIGTERM)
- Race condition testing support

### Phase 4: Health Checking ✅
- **Active Health Checks**: Periodic TCP probes (10s interval, 2s timeout)
- **Passive Health Checks**: Instant failure detection via custom transport
- Automatic backend recovery
- Smart retry logic (max 3 attempts)
- Zero-downtime failover

## Quick Start

### Prerequisites
- Go 1.21 or higher
- 3 backend servers for testing (e.g., Python HTTP servers)

### Installation

```bash
# Clone the repository
git clone https://github.com/YOUR_USERNAME/nexus-lb.git
cd nexus-lb

# Build
go build -o nexus ./cmd/nexus

# Run
./nexus
```

### Test with Backend Servers

```bash
# Terminal 1 - Backend 1
python -m http.server 8081

# Terminal 2 - Backend 2
python -m http.server 8082

# Terminal 3 - Backend 3
python -m http.server 8083

# Terminal 4 - Nexus
./nexus

# Terminal 5 - Test requests
curl http://localhost:8000
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Nexus Load Balancer                  │
│                                                          │
│  ┌────────────────────────────────────────────────┐    │
│  │              Request Handler                    │    │
│  │  • Logging                                      │    │
│  │  • Custom Headers                               │    │
│  │  • Retry Logic (max 3 attempts)                │    │
│  └─────────────────┬───────────────────────────────┘    │
│                    │                                      │
│  ┌─────────────────▼───────────────────────────────┐    │
│  │            Server Pool (RWMutex)                │    │
│  │  • Round-Robin Selection                        │    │
│  │  • Atomic Counter                               │    │
│  │  • Backend State Management                     │    │
│  └─────────────────┬───────────────────────────────┘    │
│                    │                                      │
│         ┌──────────┼──────────┐                          │
│         │          │          │                          │
│    ┌────▼───┐ ┌───▼────┐ ┌──▼─────┐                    │
│    │Backend │ │Backend │ │Backend │                     │
│    │  8081  │ │  8082  │ │  8083  │                     │
│    │        │ │        │ │        │                     │
│    │ Alive  │ │ Alive  │ │  Down  │                     │
│    └────┬───┘ └───┬────┘ └────────┘                    │
│         │         │                                      │
└─────────┼─────────┼──────────────────────────────────────┘
          │         │
    ┌─────▼─────────▼─────┐
    │   Health Checker    │
    │                     │
    │  Active (10s):      │
    │  • TCP probes       │
    │  • Recovery detect  │
    │                     │
    │  Passive (instant): │
    │  • Error detection  │
    │  • Auto-failover    │
    └─────────────────────┘
```

## Configuration

Current configuration (in `cmd/nexus/main.go`):

```go
const (
    proxyPort       = ":8000"          // Load balancer port
    healthInterval  = 10 * time.Second // Active health check interval
    healthTimeout   = 2 * time.Second  // Health check timeout
    shutdownTimeout = 30 * time.Second // Graceful shutdown timeout
    maxRetries      = 3                // Maximum retry attempts
)

var backendURLs = []string{
    "http://localhost:8081",
    "http://localhost:8082",
    "http://localhost:8083",
}
```

## Project Structure

```
nexus-lb/
├── cmd/
│   └── nexus/
│       └── main.go              # Entry point, HTTP server setup
├── internal/
│   ├── backend/
│   │   └── backend.go           # Backend representation & passive health checks
│   ├── pool/
│   │   └── pool.go              # Server pool & round-robin logic
│   └── health/
│       └── checker.go           # Active health checking
├── config/
│   └── config.go                # Configuration (placeholder for Phase 5)
├── test/
│   ├── loadtest.go              # Load testing tool
│   └── README.md                # Load testing documentation
├── go.mod                       # Go module definition
└── README.md                    # This file
```

## Load Testing

Nexus includes a built-in load testing tool:

```bash
# Build the load tester
cd test
go build -o loadtest .

# Sequential test (20 requests, 100ms delay)
./loadtest

# Concurrent test (100 requests, 10 workers, 10 req/sec)
./loadtest -c -n 100 -workers 10 -rate 10

# Stress test (1000 requests, 20 workers, 100 req/sec)
./loadtest -c -n 1000 -workers 20 -rate 100
```

See `test/README.md` for detailed load testing documentation.

## How It Works

### Round-Robin Distribution

Nexus uses an atomic counter to ensure even distribution:

```go
func (s *ServerPool) NextIndex() int {
    poolSize := len(s.backends)
    return int(atomic.AddUint64(&s.current, 1) % uint64(poolSize))
}
```

### Health Checking

**Active Health Checks** (every 10 seconds):
- TCP connection probe to each backend
- Marks backends as UP when they recover
- Logs status changes

**Passive Health Checks** (instant):
- Custom HTTP transport intercepts all requests
- Detects connection errors immediately
- Marks backend as DOWN on first failure
- Enables automatic retry with another backend

### Automatic Failover

When a backend fails:
1. **Passive check** detects error instantly → marks backend DOWN
2. **Retry logic** attempts up to 3 times with different backends
3. **Round-robin** skips DOWN backends automatically
4. **Active check** periodically tests DOWN backends for recovery

## Thread Safety

All operations are thread-safe:
- **RWMutex** protects backend slice reads/writes
- **Atomic operations** for counter increments
- **Per-backend mutex** for status updates
- **Tested with Go race detector** (`go run -race`)

## Example Output

```
2025/11/29 23:00:00 Added backend: http://localhost:8081
2025/11/29 23:00:00 Added backend: http://localhost:8082
2025/11/29 23:00:00 Added backend: http://localhost:8083
2025/11/29 23:00:00 Nexus load balancer starting on port :8000
2025/11/29 23:00:00 Load balancing across 3 backends
2025/11/29 23:00:00 Health checker starting (interval: 10s, timeout: 2s)
2025/11/29 23:00:00 Nexus is ready to accept connections

[2025-11-29 23:00:05] GET / -> http://localhost:8081 (attempt 1)
[2025-11-29 23:00:05] GET / -> http://localhost:8082 (attempt 1)
[2025-11-29 23:00:05] GET / -> http://localhost:8083 (attempt 1)

# Backend 8083 goes down...
[2025-11-29 23:00:10] GET / -> http://localhost:8083 (attempt 1)
[PASSIVE] Backend http://localhost:8083 failed: connection refused - marking as DOWN

# Subsequent requests skip 8083
[2025-11-29 23:00:11] GET / -> http://localhost:8081 (attempt 1)
[2025-11-29 23:00:11] GET / -> http://localhost:8082 (attempt 1)

# Backend 8083 recovers...
Backend http://localhost:8083 recovered (DOWN -> UP)
```

## Performance

Load test results (3 backends, 1000 requests):
```
Total Requests:      1000
Successful:          1000 (100.0%)
Failed:              0 (0.0%)
Duration:            10.2s
Requests/sec:        98.36

Backend Distribution:
  http://localhost:8081          333 (33.3%)
  http://localhost:8082          334 (33.4%)
  http://localhost:8083          333 (33.3%)
```

## Development

### Run with Race Detector

```bash
go run -race ./cmd/nexus
```

### Build for Production

```bash
go build -ldflags="-s -w" -o nexus ./cmd/nexus
```

### Run Tests

```bash
go test ./...
```

## Roadmap

- [x] **Phase 1**: Single-server reverse proxy
- [x] **Phase 2**: Round-robin load balancing
- [x] **Phase 3**: Thread safety and logging
- [x] **Phase 4**: Active & passive health checking
- [ ] **Phase 5**: Configuration management (YAML/JSON config files)
- [ ] **Phase 6**: Weighted round-robin
- [ ] **Phase 7**: Least connections algorithm
- [ ] **Phase 8**: Session persistence / sticky sessions
- [ ] **Phase 9**: Metrics and monitoring (Prometheus integration)
- [ ] **Phase 10**: TLS/HTTPS support

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Acknowledgments

Built with:
- Go standard library (`net/http`, `net/http/httputil`)
- Atomic operations for thread-safe counters
- Custom HTTP transport for passive health checking

---

**Nexus** - Simple, fast, reliable load balancing in Go 🚀
