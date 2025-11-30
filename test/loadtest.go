package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	backendCounts   map[string]*int64
	mu              sync.Mutex
	startTime       time.Time
}

func (s *Stats) recordSuccess(backend string) {
	atomic.AddInt64(&s.successRequests, 1)
	s.mu.Lock()
	if s.backendCounts[backend] == nil {
		var count int64
		s.backendCounts[backend] = &count
	}
	atomic.AddInt64(s.backendCounts[backend], 1)
	s.mu.Unlock()
}

func (s *Stats) recordFailure() {
	atomic.AddInt64(&s.failedRequests, 1)
}

func (s *Stats) printSummary() {
	duration := time.Since(s.startTime)
	total := atomic.LoadInt64(&s.totalRequests)
	success := atomic.LoadInt64(&s.successRequests)
	failed := atomic.LoadInt64(&s.failedRequests)

	fmt.Println("\n" + "==============================================")
	fmt.Println("LOAD TEST SUMMARY")
	fmt.Println("==============================================")
	fmt.Printf("Total Requests:      %d\n", total)
	fmt.Printf("Successful:          %d (%.1f%%)\n", success, float64(success)/float64(total)*100)
	fmt.Printf("Failed:              %d (%.1f%%)\n", failed, float64(failed)/float64(total)*100)
	fmt.Printf("Duration:            %v\n", duration.Round(time.Millisecond))
	fmt.Printf("Requests/sec:        %.2f\n", float64(total)/duration.Seconds())
	fmt.Println("----------------------------------------------")
	fmt.Println("Backend Distribution:")

	s.mu.Lock()
	for backend, count := range s.backendCounts {
		c := atomic.LoadInt64(count)
		fmt.Printf("  %-30s %d (%.1f%%)\n", backend, c, float64(c)/float64(success)*100)
	}
	s.mu.Unlock()
	fmt.Println("==============================================")
}

func sendRequest(url string, requestNum int, stats *Stats) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("[%d] FAILED: %v\n", requestNum, err)
		stats.recordFailure()
		return
	}
	defer resp.Body.Close()

	// Read the response body
	io.Copy(io.Discard, resp.Body)

	// Get the backend server from the custom header
	backend := resp.Header.Get("X-Backend-Server")
	if backend == "" {
		backend = "Unknown"
	}

	fmt.Printf("[%d] SUCCESS: Status=%d, Backend=%s\n", requestNum, resp.StatusCode, backend)
	stats.recordSuccess(backend)
}

func runSequentialTest(url string, numRequests int, delayMs int) {
	fmt.Printf("Starting SEQUENTIAL load test\n")
	fmt.Printf("Target URL:     %s\n", url)
	fmt.Printf("Total Requests: %d\n", numRequests)
	fmt.Printf("Delay:          %dms between requests\n\n", delayMs)

	stats := &Stats{
		totalRequests: int64(numRequests),
		backendCounts: make(map[string]*int64),
		startTime:     time.Now(),
	}

	for i := 1; i <= numRequests; i++ {
		sendRequest(url, i, stats)
		if delayMs > 0 && i < numRequests {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	stats.printSummary()
}

func runConcurrentTest(url string, numRequests, concurrency int, rateLimit int) {
	fmt.Printf("Starting CONCURRENT load test\n")
	fmt.Printf("Target URL:     %s\n", url)
	fmt.Printf("Total Requests: %d\n", numRequests)
	fmt.Printf("Concurrency:    %d goroutines\n", concurrency)
	fmt.Printf("Rate Limit:     %d requests/second\n\n", rateLimit)

	stats := &Stats{
		totalRequests: int64(numRequests),
		backendCounts: make(map[string]*int64),
		startTime:     time.Now(),
	}

	var wg sync.WaitGroup
	requestChan := make(chan int, numRequests)

	// Rate limiter: controls how fast requests are sent
	ticker := time.NewTicker(time.Second / time.Duration(rateLimit))
	defer ticker.Stop()

	// Start worker goroutines
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for requestNum := range requestChan {
				sendRequest(url, requestNum, stats)
			}
		}(i)
	}

	// Send requests with rate limiting
	go func() {
		for i := 1; i <= numRequests; i++ {
			<-ticker.C // Wait for rate limiter
			requestChan <- i
		}
		close(requestChan)
	}()

	wg.Wait()
	stats.printSummary()
}

func main() {
	// Command line flags
	url := flag.String("url", "http://localhost:8000", "Target URL")
	numRequests := flag.Int("n", 20, "Number of requests")
	concurrent := flag.Bool("c", false, "Run concurrent test (default: sequential)")
	concurrency := flag.Int("workers", 10, "Number of concurrent workers (only for -c)")
	rateLimit := flag.Int("rate", 10, "Requests per second (only for -c)")
	delayMs := flag.Int("delay", 100, "Delay between requests in ms (only for sequential)")

	flag.Parse()

	if *concurrent {
		runConcurrentTest(*url, *numRequests, *concurrency, *rateLimit)
	} else {
		runSequentialTest(*url, *numRequests, *delayMs)
	}
}
