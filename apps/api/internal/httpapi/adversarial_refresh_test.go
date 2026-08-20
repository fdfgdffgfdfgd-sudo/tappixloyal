package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestAdversarialRefreshTokenConcurrentCallsRemainSingleUse(t *testing.T) {
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("set TEST_REDIS_ADDR to run refresh lifecycle test")
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })
	a := &api{redis: client, jwtSecret: []byte("refresh-lifecycle-test-secret")}
	initialRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	_, refreshToken, err := a.issueTokens(initialRequest, "refresh-test-user", "refresh-test-company", "company_owner")
	if err != nil {
		t.Fatal(err)
	}
	oldKey := "refresh:" + tokenHash(refreshToken)
	t.Cleanup(func() { _ = client.Del(context.Background(), oldKey).Err() })
	if exists, err := client.Exists(t.Context(), oldKey).Result(); err != nil || exists != 1 {
		t.Fatalf("test session was not created: exists=%d err=%v", exists, err)
	}

	const callers = 2
	ready := sync.WaitGroup{}
	ready.Add(callers)
	start := make(chan struct{})
	workers := sync.WaitGroup{}
	workers.Add(callers)
	responses := make(chan *httptest.ResponseRecorder, callers)
	for range callers {
		go func() {
			defer workers.Done()
			ready.Done()
			<-start
			body, _ := json.Marshal(refreshInput{RefreshToken: refreshToken})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			a.refresh(rec, req)
			responses <- rec
		}()
	}
	ready.Wait()
	close(start)
	workers.Wait()
	close(responses)

	codes := make([]int, 0, callers)
	newTokens := make([]string, 0, callers)
	for rec := range responses {
		codes = append(codes, rec.Code)
		if rec.Code != http.StatusOK {
			continue
		}
		var envelope struct {
			Success bool `json:"success"`
			Data    struct {
				RefreshToken string `json:"refreshToken"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if !envelope.Success || envelope.Data.RefreshToken == "" {
			t.Fatalf("successful refresh did not return a new token: %s", rec.Body.String())
		}
		newTokens = append(newTokens, envelope.Data.RefreshToken)
	}
	sort.Ints(codes)
	oldExists, err := client.Exists(t.Context(), oldKey).Result()
	if err != nil {
		t.Fatal(err)
	}
	newSessionCount := 0
	for _, token := range newTokens {
		if exists, e := client.Exists(t.Context(), "refresh:"+tokenHash(token)).Result(); e != nil {
			t.Fatal(e)
		} else if exists == 1 {
			newSessionCount++
		}
	}
	if len(newTokens) != 1 || codes[0] != http.StatusOK || codes[1] != http.StatusUnauthorized || oldExists != 0 || newSessionCount != 1 {
		t.Fatalf("single-use refresh invariant violated: response_codes=%v new_tokens=%d live_new_sessions=%d old_token_exists=%d", codes, len(newTokens), newSessionCount, oldExists)
	}
}
