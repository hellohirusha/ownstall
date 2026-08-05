package telemetry

import (
	"net"
	"net/http"
	"strings"
)

// responseWriter captures the status code so middleware can report it
// after the handler has run.
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func wrapWriter(w http.ResponseWriter) *responseWriter {
	// 200 is the status net/http assumes when a handler writes a body
	// without calling WriteHeader.
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.wroteHeader = true
	return rw.ResponseWriter.Write(b)
}

// ClientIP resolves the caller's address behind Railway's proxy.
//
// X-Forwarded-For accumulates a list as a request crosses proxies, and
// the client may have sent one of its own; the leftmost entry added by
// our edge is the closest thing to the real caller. Taking the whole
// header instead would let a client mint unlimited distinct rate-limit
// buckets just by varying it.
func ClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if first != "" {
			return first
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
