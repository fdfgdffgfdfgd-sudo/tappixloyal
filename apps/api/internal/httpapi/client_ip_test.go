package httpapi

import (
	"net/http"
	"testing"
)

func request(remote string, headers map[string]string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestClientIPBehindProxy(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		headers map[string]string
		want    string
	}{
		{
			// Without this the whole platform shares one rate-limit bucket,
			// because every proxied request arrives from the same address.
			name:    "takes the caller Nginx saw",
			remote:  "172.19.0.7:41234",
			headers: map[string]string{"X-Real-IP": "203.0.113.9"},
			want:    "203.0.113.9",
		},
		{
			name:    "falls back to the entry our proxy appended",
			remote:  "10.1.2.3:5000",
			headers: map[string]string{"X-Forwarded-For": "198.51.100.7, 203.0.113.9"},
			want:    "203.0.113.9",
		},
		{
			// A request straight off the internet must not be able to claim
			// somebody else's bucket by sending its own header.
			name:    "ignores a header sent by a direct caller",
			remote:  "203.0.113.50:5000",
			headers: map[string]string{"X-Real-IP": "10.0.0.1", "X-Forwarded-For": "10.0.0.1"},
			want:    "203.0.113.50",
		},
		{
			name:   "uses the peer when the proxy sent no header",
			remote: "172.19.0.7:41234",
			want:   "172.19.0.7",
		},
		{
			name:    "ignores an empty header",
			remote:  "127.0.0.1:8080",
			headers: map[string]string{"X-Real-IP": "   "},
			want:    "127.0.0.1",
		},
		{
			name:   "survives an address with no port",
			remote: "203.0.113.50",
			want:   "203.0.113.50",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clientIP(request(c.remote, c.headers)); got != c.want {
				t.Fatalf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}
