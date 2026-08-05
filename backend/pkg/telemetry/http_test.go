package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ClientIP is the key the per-IP rate limiter counts against, so which
// header wins decides whose budget a request spends.
func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "no proxy headers uses the socket address",
			remoteAddr: "203.0.113.7:54321",
			want:       "203.0.113.7",
		},
		{
			// Railway sits in front of the API, so the socket address is
			// the proxy and the left-most forwarded entry is the client.
			name:       "x-forwarded-for takes the left-most entry",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.7, 70.41.3.18, 10.0.0.1"},
			want:       "203.0.113.7",
		},
		{
			name:       "x-forwarded-for is trimmed",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "  203.0.113.7  ,10.0.0.1"},
			want:       "203.0.113.7",
		},
		{
			name:       "falls through an empty x-forwarded-for to x-real-ip",
			remoteAddr: "10.0.0.1:443",
			headers: map[string]string{
				"X-Forwarded-For": "  ",
				"X-Real-IP":       "203.0.113.9",
			},
			want: "203.0.113.9",
		},
		{
			name:       "x-forwarded-for wins over x-real-ip",
			remoteAddr: "10.0.0.1:443",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.7",
				"X-Real-IP":       "198.51.100.4",
			},
			want: "203.0.113.7",
		},
		{
			name:       "ipv6 socket address drops the port",
			remoteAddr: "[2001:db8::1]:8080",
			want:       "2001:db8::1",
		},
		{
			// SplitHostPort fails without a port; returning the raw value
			// still yields a stable rate-limit key.
			name:       "address without a port is returned as-is",
			remoteAddr: "203.0.113.7",
			want:       "203.0.113.7",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/query", nil)
			req.RemoteAddr = tc.remoteAddr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			if got := ClientIP(req); got != tc.want {
				t.Errorf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

// The wrapper records the status for metrics and logging. A handler
// that writes a body without calling WriteHeader has implicitly sent
// 200, and the wrapper must report that rather than a zero status.
func TestWrapWriter_DefaultsTo200(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := wrapWriter(rec)

	if _, err := rw.Write([]byte("body")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if rw.statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusOK)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("underlying recorder status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWrapWriter_RecordsExplicitStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := wrapWriter(rec)

	rw.WriteHeader(http.StatusTooManyRequests)

	if rw.statusCode != http.StatusTooManyRequests {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusTooManyRequests)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("underlying recorder status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

// net/http logs "superfluous WriteHeader" and ignores the second call,
// so the recorded status must stay on the first one written.
func TestWrapWriter_IgnoresSecondWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := wrapWriter(rec)

	rw.WriteHeader(http.StatusUnauthorized)
	rw.WriteHeader(http.StatusOK)

	if rw.statusCode != http.StatusUnauthorized {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusUnauthorized)
	}
}
