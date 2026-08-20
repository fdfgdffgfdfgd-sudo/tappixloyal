// loadvalidate is a local-only end-to-end fixture validator. It uses normal HTTP
// authentication and never creates JWTs or Redis sessions itself.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type manifest struct {
	Version             int       `json:"version"`
	GeneratedAt         time.Time `json:"generatedAt"`
	UserCount           int       `json:"userCount"`
	CompanyCount        int       `json:"companyCount"`
	GuestFixtureCount   int       `json:"guestFixtureCount"`
	BookingFixtureCount int       `json:"bookingFixtureCount"`
	BaseURL             string    `json:"baseUrl"`
	Email               []string  `json:"emails"`
	Password            string    `json:"password"`
	Guest               []string  `json:"guestTokens"`
	Slugs               []string  `json:"siteSlugs"`
	Slots               []string  `json:"bookingSlots"`
	Branches            []string  `json:"branchIds"`
}
type envelope struct {
	Success bool `json:"success"`
	Data    struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	} `json:"data"`
}

func main() {
	path := flag.String("manifest", "manifest.json", "testseed manifest")
	flag.Parse()
	if os.Getenv("TAPPIX_TEST_ENV") != "1" {
		fail("refusing validation outside TAPPIX_TEST_ENV=1")
	}
	raw, err := os.ReadFile(*path)
	if err != nil {
		fail(err.Error())
	}
	var m manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		fail(err.Error())
	}
	if m.Version != 1 || m.BaseURL == "" || m.UserCount != len(m.Email) || m.CompanyCount < 1 || m.GuestFixtureCount != len(m.Guest) || m.BookingFixtureCount != len(m.Slots) || len(m.Slots) == 0 || m.GeneratedAt.IsZero() {
		fail("manifest metadata or fixture counts are invalid")
	}
	seen := map[string]bool{}
	for _, email := range m.Email {
		if seen[email] {
			fail("duplicate user credential in manifest")
		}
		seen[email] = true
	}
	seen = map[string]bool{}
	for _, slot := range m.Slots {
		if seen[slot] {
			fail("duplicate booking slot in manifest")
		}
		seen[slot] = true
	}
	if m.BaseURL == "" || len(m.Email) == 0 || len(m.Guest) == 0 {
		fail("manifest lacks baseUrl, users or guest tokens")
	}
	client := &http.Client{}
	loginBody, _ := json.Marshal(map[string]string{"email": m.Email[0], "password": m.Password})
	status, body := request(client, http.MethodPost, m.BaseURL+"/api/v1/auth/login", loginBody, "")
	if status != http.StatusOK {
		fail(fmt.Sprintf("login: HTTP %d: %s", status, body))
	}
	var login envelope
	if json.Unmarshal([]byte(body), &login) != nil || !login.Success || login.Data.AccessToken == "" || login.Data.RefreshToken == "" {
		fail("login response did not contain access and refresh tokens")
	}
	status, body = request(client, http.MethodGet, m.BaseURL+"/api/v1/auth/me", nil, "Bearer "+login.Data.AccessToken)
	if status != http.StatusOK {
		fail(fmt.Sprintf("auth/me: HTTP %d: %s", status, body))
	}
	refreshBody, _ := json.Marshal(map[string]string{"refreshToken": login.Data.RefreshToken})
	status, body = request(client, http.MethodPost, m.BaseURL+"/api/v1/auth/refresh", refreshBody, "")
	if status != http.StatusOK {
		fail(fmt.Sprintf("refresh: HTTP %d: %s", status, body))
	}
	status, body = request(client, http.MethodGet, m.BaseURL+"/api/v1/public/guest/"+m.Guest[0], nil, "")
	if status != http.StatusOK {
		fail(fmt.Sprintf("guest: HTTP %d: %s", status, body))
	}
	status, body = request(client, http.MethodGet, m.BaseURL+"/api/v1/reward-definitions", nil, "Bearer "+login.Data.AccessToken)
	if status != http.StatusOK {
		fail(fmt.Sprintf("loyalty read: HTTP %d: %s", status, body))
	}
	writeBody := []byte(`{"name":"Validation reward","description":"local validation","rewardType":"gift","value":0,"repeatable":true,"cooldownDays":0,"confirmationMethod":"staff","branchIds":[]}`)
	status, body = request(client, http.MethodPost, m.BaseURL+"/api/v1/reward-definitions", writeBody, "Bearer "+login.Data.AccessToken)
	if status < 200 || status >= 300 {
		fail(fmt.Sprintf("loyalty write: HTTP %d: %s", status, body))
	}
	if len(m.Slugs) == 0 || len(m.Slots) == 0 {
		fail("manifest lacks booking site or slots")
	}
	if len(m.Branches) == 0 {
		fail("manifest lacks booking branches")
	}
	booking, _ := json.Marshal(map[string]string{"branchId": m.Branches[0], "customerName": "Load Validation", "phone": "+77008887766", "service": "Consultation", "startsAt": m.Slots[0]})
	status, body = request(client, http.MethodPost, m.BaseURL+"/api/v1/public/sites/"+m.Slugs[0]+"/bookings", booking, "")
	if status < 200 || status >= 300 {
		fail(fmt.Sprintf("booking create: HTTP %d: %s", status, body))
	}
	fmt.Println("PASS login auth/me refresh guest loyalty-read loyalty-write booking-create")
}

func request(client *http.Client, method, url string, body []byte, auth string) (int, string) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		fail(err.Error())
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := client.Do(req)
	if err != nil {
		fail(err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, strings.TrimSpace(string(b))
}
func fail(message string) { fmt.Fprintln(os.Stderr, "VALIDATION FAILED:", message); os.Exit(1) }
