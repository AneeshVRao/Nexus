# Nexus Load Testing

Safe load testing tool for the Nexus load balancer.

## Quick Start

### 1. Start Backend Servers (3 terminals)
```powershell
python -m http.server 8081
python -m http.server 8082
python -m http.server 8083
```

### 2. Start Nexus
```powershell
# Regular mode
go run ./cmd/nexus

# With race detection (recommended for testing)
go run -race ./cmd/nexus
```

### 3. Run Load Tests

## Sequential Tests (Safest)

**Basic sequential test (20 requests, 100ms delay):**
```powershell
.\test\loadtest.exe
```

**Custom sequential test:**
```powershell
# 50 requests with 50ms delay
.\test\loadtest.exe -n 50 -delay 50

# 100 requests with no delay
.\test\loadtest.exe -n 100 -delay 0
```

## Concurrent Tests (Use Carefully)

**Safe concurrent test (10 workers, 100 requests, 10 req/sec):**
```powershell
.\test\loadtest.exe -c -n 100 -workers 10 -rate 10
```

**Moderate load (10 workers, 500 requests, 50 req/sec):**
```powershell
.\test\loadtest.exe -c -n 500 -workers 10 -rate 50
```

**Higher load (20 workers, 1000 requests, 100 req/sec):**
```powershell
.\test\loadtest.exe -c -n 1000 -workers 20 -rate 100
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | http://localhost:8000 | Target URL |
| `-n` | 20 | Number of requests |
| `-c` | false | Enable concurrent mode |
| `-workers` | 10 | Number of concurrent workers (concurrent mode only) |
| `-rate` | 10 | Requests per second (concurrent mode only) |
| `-delay` | 100 | Delay between requests in ms (sequential mode only) |

## Recommended Safe Tests

### For Development Machine:

1. **Gentle test** (Sequential, no system stress):
   ```powershell
   .\test\loadtest.exe -n 50 -delay 100
   ```

2. **Light concurrent** (Safe for most machines):
   ```powershell
   .\test\loadtest.exe -c -n 100 -workers 5 -rate 10
   ```

3. **Moderate concurrent** (Watch CPU usage):
   ```powershell
   .\test\loadtest.exe -c -n 500 -workers 10 -rate 50
   ```

4. **Stress test** (Only if machine is powerful):
   ```powershell
   .\test\loadtest.exe -c -n 2000 -workers 20 -rate 100
   ```

## What to Monitor

- **CPU Usage**: Should stay below 80%
- **Memory**: Watch for leaks
- **Race Detector**: Any "DATA RACE" warnings mean bugs
- **Backend Distribution**: Should be roughly equal across all backends

## Sample Output

```
Starting CONCURRENT load test
Target URL:     http://localhost:8000
Total Requests: 100
Concurrency:    10 goroutines
Rate Limit:     10 requests/second

[1] SUCCESS: Status=200, Backend=http://localhost:8081
[2] SUCCESS: Status=200, Backend=http://localhost:8082
[3] SUCCESS: Status=200, Backend=http://localhost:8083
...

==============================================
LOAD TEST SUMMARY
==============================================
Total Requests:      100
Successful:          100 (100.0%)
Failed:              0 (0.0%)
Duration:            10.2s
Requests/sec:        9.80
----------------------------------------------
Backend Distribution:
  http://localhost:8081          34 (34.0%)
  http://localhost:8082          33 (33.0%)
  http://localhost:8083          33 (33.0%)
==============================================
```

## Troubleshooting

**"connection refused" errors:**
- Make sure backend servers are running on ports 8081-8083

**VS Code/system crashes:**
- Reduce `-workers` (try 5 instead of 10)
- Reduce `-rate` (try 10 instead of 50)
- Reduce `-n` (try 100 instead of 1000)
- Use sequential mode instead of concurrent

**Uneven backend distribution:**
- Normal with small numbers (<50 requests)
- Should even out with more requests (>500)
