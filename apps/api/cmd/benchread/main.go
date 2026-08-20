package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	lat   int64
	code  int
	bytes int64
}

func main() {
	base := flag.String("base", "http://localhost:8080", "")
	token := flag.String("token", "", "")
	vus := flag.Int("vus", 1, "")
	iterations := flag.Int("iterations", 20, "")
	flag.Parse()
	if *vus < 1 || *iterations < 1 {
		panic("vus and iterations must be positive")
	}
	var mu sync.Mutex
	all := []result{}
	var done int64
	start := time.Now()
	var wg sync.WaitGroup
	for v := 0; v < *vus; v++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &http.Client{Timeout: 15 * time.Second}
			for i := 0; i < *iterations; i++ {
				t := time.Now()
				req, _ := http.NewRequest(http.MethodGet, *base+"/api/v1/reward-definitions", nil)
				req.Header.Set("Authorization", "Bearer "+*token)
				resp, e := c.Do(req)
				if e != nil {
					mu.Lock()
					all = append(all, result{lat: time.Since(t).Microseconds(), code: 599})
					mu.Unlock()
					continue
				}
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				mu.Lock()
				all = append(all, result{lat: time.Since(t).Microseconds(), code: resp.StatusCode, bytes: int64(len(b))})
				mu.Unlock()
				atomic.AddInt64(&done, 1)
			}
		}()
	}
	wg.Wait()
	sort.Slice(all, func(i, j int) bool { return all[i].lat < all[j].lat })
	pct := func(p int) int64 {
		if len(all) == 0 {
			return 0
		}
		return all[(len(all)-1)*p/100].lat / 1000
	}
	errors := 0
	bytes := int64(0)
	for _, r := range all {
		if r.code < 200 || r.code >= 300 {
			errors++
		}
		bytes += r.bytes
	}
	secs := time.Since(start).Seconds()
	fmt.Printf("vus=%d requests=%d rps=%.2f p50_ms=%d p95_ms=%d p99_ms=%d max_ms=%d errors=%d response_bytes=%d\n", *vus, len(all), float64(len(all))/secs, pct(50), pct(95), pct(99), all[len(all)-1].lat/1000, errors, bytes)
	_ = json.Valid
}
