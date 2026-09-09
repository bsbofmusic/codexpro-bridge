package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

type result struct {
	duration time.Duration
	err      error
}

func tokenFromEnv() string {
	if value := os.Getenv("CODEXPRO_BRIDGE_HTTP_TOKEN"); value != "" {
		return value
	}
	return os.Getenv("CODEXPRO_HTTP_TOKEN")
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	idx := int(float64(len(values)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:18787/mcp", "Bridge MCP endpoint")
	mode := flag.String("mode", "surface", "surface or tool")
	tool := flag.String("tool", "", "read-only Bridge tool for tool mode")
	arguments := flag.String("arguments", "{}", "JSON object for tool mode")
	requests := flag.Int("requests", 100, "total requests")
	concurrency := flag.Int("concurrency", 8, "parallel workers")
	timeout := flag.Duration("timeout", 30*time.Second, "per-operation timeout")
	flag.Parse()

	if *requests < 1 || *concurrency < 1 || *concurrency > 128 {
		fmt.Fprintln(os.Stderr, "invalid requests/concurrency")
		os.Exit(2)
	}
	if *mode != "surface" && *mode != "tool" {
		fmt.Fprintln(os.Stderr, "mode must be surface or tool")
		os.Exit(2)
	}
	if *mode == "tool" && *tool == "" {
		fmt.Fprintln(os.Stderr, "tool mode requires -tool")
		os.Exit(2)
	}

	token := tokenFromEnv()
	if token == "" {
		fmt.Fprintln(os.Stderr, "Bridge token is not configured in the process environment")
		os.Exit(2)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(*arguments), &args); err != nil || args == nil {
		fmt.Fprintln(os.Stderr, "arguments must be a JSON object")
		os.Exit(2)
	}

	transport := bearerTransport{token: token, base: http.DefaultTransport}
	httpClient := &http.Client{Transport: transport}

	jobs := make(chan struct{})
	results := make(chan result, *requests)
	var started atomic.Int64
	var wg sync.WaitGroup
	workerCount := *concurrency
	if workerCount > *requests {
		workerCount = *requests
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				started.Add(1)
				begin := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), *timeout)
				err := runOne(ctx, httpClient, *endpoint, *mode, *tool, args)
				cancel()
				results <- result{duration: time.Since(begin), err: err}
			}
		}()
	}

	begin := time.Now()
	go func() {
		for i := 0; i < *requests; i++ {
			jobs <- struct{}{}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	latencies := make([]time.Duration, 0, *requests)
	failures := 0
	firstErrors := make([]string, 0, 5)
	for r := range results {
		latencies = append(latencies, r.duration)
		if r.err != nil {
			failures++
			if len(firstErrors) < cap(firstErrors) {
				firstErrors = append(firstErrors, r.err.Error())
			}
		}
	}
	total := time.Since(begin)
	success := len(latencies) - failures
	opsPerSecond := float64(len(latencies)) / total.Seconds()

	fmt.Printf("mode=%s requests=%d concurrency=%d success=%d failures=%d elapsed=%s ops_per_sec=%.2f p50=%s p95=%s p99=%s max=%s\n",
		*mode, len(latencies), workerCount, success, failures, total.Round(time.Millisecond), opsPerSecond,
		percentile(append([]time.Duration(nil), latencies...), 0.50).Round(time.Millisecond),
		percentile(append([]time.Duration(nil), latencies...), 0.95).Round(time.Millisecond),
		percentile(append([]time.Duration(nil), latencies...), 0.99).Round(time.Millisecond),
		percentile(append([]time.Duration(nil), latencies...), 1.0).Round(time.Millisecond),
	)
	for _, msg := range firstErrors {
		fmt.Printf("error=%q\n", msg)
	}
	if failures != 0 {
		os.Exit(1)
	}
}

func runOne(ctx context.Context, httpClient *http.Client, endpoint, mode, tool string, args map[string]any) error {
	client := mcp.NewClient(&mcp.Implementation{Name: "bridge-stress", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           httpClient,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer session.Close()

	switch mode {
	case "surface":
		_, err = session.ListTools(ctx, nil)
		if err != nil {
			return fmt.Errorf("tools/list: %w", err)
		}
		return nil
	case "tool":
		_, err = session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			return fmt.Errorf("tools/call %s: %w", tool, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported mode")
	}
}
