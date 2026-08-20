// loadrun is a local-only mixed HTTP workload runner for testseed manifests.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type manifest struct {
	Version             int      `json:"version"`
	UserCount           int      `json:"userCount"`
	CompanyCount        int      `json:"companyCount"`
	GuestFixtureCount   int      `json:"guestFixtureCount"`
	BookingFixtureCount int      `json:"bookingFixtureCount"`
	BaseURL             string   `json:"baseUrl"`
	Emails              []string `json:"emails"`
	Password            string   `json:"password"`
	Guest               []string `json:"guestTokens"`
	Slugs               []string `json:"siteSlugs"`
	Branches            []string `json:"branchIds"`
}
type tokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}
type envelope struct {
	Data tokenData `json:"data"`
}
type sample struct {
	lat  int64
	code int
	kind string
}
type stats struct {
	mu  sync.Mutex
	all []sample
}

func main() {
	if os.Getenv("TAPPIX_TEST_ENV") != "1" {
		fail("set TAPPIX_TEST_ENV=1")
	}
	path := flag.String("manifest", "manifest.json", "")
	users := flag.Int("users", 10, "")
	iters := flag.Int("iterations", 20, "")
	flag.Parse()
	b, e := os.ReadFile(*path)
	if e != nil {
		fail(e.Error())
	}
	var m manifest
	if e = json.Unmarshal(b, &m); e != nil {
		fail(e.Error())
	}
	if len(m.Emails) == 0 || len(m.Guest) == 0 || len(m.Slugs) == 0 || len(m.Branches) == 0 {
		fail("manifest lacks users, guest, site or branch")
	}
	if m.Version != 1 || m.UserCount != len(m.Emails) || m.GuestFixtureCount != len(m.Guest) || m.BookingFixtureCount < *users {
		fail("manifest is invalid or has insufficient independent fixtures")
	}
	for _, n := range []int{*users} {
		if n < 1 || n > 500 {
			fail("users must be 1..500")
		}
	}
	var s stats
	type session struct{ access, refresh string }
	sessions := make([]session, *users)
	for u := 0; u < *users; u++ {
		a, r := login(&http.Client{}, m.BaseURL, m.Emails[u], m.Password, nil)
		if a == "" || r == "" {
			fail(fmt.Sprintf("bootstrap login failed for user %d", u))
		}
		sessions[u] = session{a, r}
	}
	var wg sync.WaitGroup
	for u := 0; u < *users; u++ {
		wg.Add(1)
		go func(v int) { defer wg.Done(); runVU(&m, v, *iters, sessions[v].access, sessions[v].refresh, &s) }(u)
	}
	wg.Wait()
	report(s.all)
}
func runVU(m *manifest, v, iters int, access, refresh string, s *stats) {
	client := &http.Client{}
	for i := 0; i < iters; i++ {
		// Twenty-operation measured cycle: 7 guest, 4 me, 3 refresh,
		// 3 loyalty reads, 2 writes and 1 booking. Bootstrap login is excluded.
		x := i % 20
		switch {
		case x < 7:
			req(client, m.BaseURL+"/api/v1/public/guest/"+m.Guest[v%len(m.Guest)], "", http.MethodGet, nil, "guest", s)
		case x < 11:
			req(client, m.BaseURL+"/api/v1/auth/me", access, http.MethodGet, nil, "me", s)
		case x < 14:
			body := []byte(fmt.Sprintf(`{"refreshToken":%q}`, refresh))
			code, raw, latency := do(client, m.BaseURL+"/api/v1/auth/refresh", http.MethodPost, body, "")
			s.add(code, "refresh", raw, latency)
			if code == 200 {
				var out envelope
				if json.Unmarshal(raw, &out) == nil && out.Data.RefreshToken != "" {
					access, refresh = out.Data.AccessToken, out.Data.RefreshToken
				}
			}
		case x < 16:
			req(client, m.BaseURL+"/api/v1/reward-definitions", access, http.MethodGet, nil, "loyalty_read", s)
		case x < 18:
			body := []byte(fmt.Sprintf(`{"name":"load-%d-%d","description":"local","rewardType":"gift","value":0,"repeatable":true,"cooldownDays":0,"confirmationMethod":"staff","branchIds":[]}`, v, i))
			req(client, m.BaseURL+"/api/v1/reward-definitions", access, http.MethodPost, body, "loyalty_write", s)
		default:
			body := []byte(fmt.Sprintf(`{"branchId":%q,"customerName":"Load %d","phone":"+7700%08d","service":"Consultation","startsAt":"%s"}`, m.Branches[v%len(m.Branches)], v, v*10000+i, time.Now().UTC().Add(time.Duration(24+v*2+i)*time.Hour).Format(time.RFC3339)))
			req(client, m.BaseURL+"/api/v1/public/sites/"+m.Slugs[v%len(m.Slugs)]+"/bookings", "", http.MethodPost, body, "booking", s)
		}
	}
}
func login(c *http.Client, base, email, password string, s *stats) (string, string) {
	var out envelope
	body := []byte(fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
	code, b, latency := do(c, base+"/api/v1/auth/login", http.MethodPost, body, "")
	if s != nil {
		s.add(code, "login", b, latency)
	}
	if code != 200 || json.Unmarshal(b, &out) != nil {
		return "", ""
	}
	return out.Data.AccessToken, out.Data.RefreshToken
}
func req(c *http.Client, url, access, method string, body []byte, kind string, s *stats) int {
	code, b, latency := do(c, url, method, body, access)
	s.add(code, kind, b, latency)
	return code
}
func do(c *http.Client, url, method string, body []byte, access string) (int, []byte, int64) {
	start := time.Now()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	q, e := http.NewRequest(method, url, r)
	if e != nil {
		return 599, nil, time.Since(start).Microseconds()
	}
	if body != nil {
		q.Header.Set("Content-Type", "application/json")
	}
	if access != "" {
		q.Header.Set("Authorization", "Bearer "+access)
	}
	resp, e := c.Do(q)
	if e != nil {
		return 599, nil, time.Since(start).Microseconds()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = time.Since(start)
	return resp.StatusCode, b, time.Since(start).Microseconds()
}
func (s *stats) add(code int, k string, _ []byte, latency int64) {
	s.mu.Lock()
	s.all = append(s.all, sample{code: code, kind: k, lat: latency})
	s.mu.Unlock()
}
func report(a []sample) {
	sort.Slice(a, func(i, j int) bool { return a[i].code < a[j].code })
	counts := map[string][4]int{}
	for _, x := range a {
		v := counts[x.kind]
		v[0]++
		if x.code >= 200 && x.code < 300 {
			v[1]++
		}
		if x.code == 429 {
			v[2]++
		}
		if x.code >= 500 || x.code == 599 {
			v[3]++
		}
		counts[x.kind] = v
	}
	fmt.Println("kind,total,success,429,5xx,p50_ms,p95_ms,p99_ms,max_ms")
	for k, v := range counts {
		var values []int64
		for _, x := range a {
			if x.kind == k {
				values = append(values, x.lat)
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		pct := func(p int) int64 {
			if len(values) == 0 {
				return 0
			}
			return values[(len(values)-1)*p/100]
		}
		max := int64(0)
		if len(values) > 0 {
			max = values[len(values)-1]
		}
		fmt.Printf("%s,%d,%d,%d,%d,%d,%d,%d,%d\n", k, v[0], v[1], v[2], v[3], pct(50)/1000, pct(95)/1000, pct(99)/1000, max/1000)
	}
}
func fail(s string) { fmt.Fprintln(os.Stderr, "LOAD FAILED:", s); os.Exit(1) }
