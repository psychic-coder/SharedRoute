package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pterm/pterm"
)

var (
	targetURL   = flag.String("url", "http://localhost:8081/v1/check", "Target URL")
	concurrency = flag.Int("c", 50, "Concurrency level")
	duration    = flag.Duration("d", 10*time.Second, "Test duration")
)

func main() {
	flag.Parse()

	pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).WithTextStyle(pterm.NewStyle(pterm.FgBlack)).Println("ShardRoute Benchmark Tool")
	pterm.Info.Printf("Target: %s\n", *targetURL)
	pterm.Info.Printf("Concurrency: %d\n", *concurrency)
	pterm.Info.Printf("Duration: %s\n\n", *duration)

	var allowed, rejected, errors uint64
	
	area, _ := pterm.DefaultArea.Start()
	defer area.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			client := &http.Client{Timeout: 2 * time.Second}
			payload, _ := json.Marshal(map[string]any{"key": fmt.Sprintf("user_%d", workerID), "cost": 1, "limit_name": "api"})
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					req, _ := http.NewRequest("POST", *targetURL, bytes.NewBuffer(payload))
					req.Header.Set("Content-Type", "application/json")
					resp, err := client.Do(req)
					if err != nil {
						atomic.AddUint64(&errors, 1)
						continue
					}
					if resp.StatusCode == 200 {
						atomic.AddUint64(&allowed, 1)
					} else if resp.StatusCode == 429 {
						atomic.AddUint64(&rejected, 1)
					} else {
						atomic.AddUint64(&errors, 1)
					}
					resp.Body.Close()
				}
			}
		}(i)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
				a := atomic.LoadUint64(&allowed)
				r := atomic.LoadUint64(&rejected)
				e := atomic.LoadUint64(&errors)
				elapsed := time.Since(start).Seconds()
				rps := float64(a+r+e) / elapsed

				content := pterm.Sprintf(
					"Time Elapsed: %0.1fs / %0.1fs\n"+
						"Requests/sec: %.2f\n"+
						"Allowed: %s | Rejected: %s | Errors: %s",
					elapsed, duration.Seconds(),
					rps,
					pterm.Green(a), pterm.Yellow(r), pterm.Red(e),
				)
				area.Update(content)
			}
		}
	}()

	wg.Wait()
	area.Stop()

	pterm.Success.Println("Benchmark Complete!")
	a := atomic.LoadUint64(&allowed)
	r := atomic.LoadUint64(&rejected)
	e := atomic.LoadUint64(&errors)
	elapsed := time.Since(start).Seconds()
	
	pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
		{"Metric", "Value"},
		{"Total Requests", fmt.Sprintf("%d", a+r+e)},
		{"Requests/sec", fmt.Sprintf("%.2f", float64(a+r+e)/elapsed)},
		{"Allowed", fmt.Sprintf("%d", a)},
		{"Rejected", fmt.Sprintf("%d", r)},
		{"Errors", fmt.Sprintf("%d", e)},
	}).Render()
}
