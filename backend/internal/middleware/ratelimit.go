package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hellohirusha/ownstall/pkg/telemetry"
)

// RateLimiter counts requests per key in a fixed window held in Redis.
//
// Fixed window, not sliding: it costs one round trip instead of a
// sorted-set read, and the worst case (a client sending 2x the limit
// across a window boundary) is acceptable for abuse control on a
// storefront API.
type RateLimiter struct {
	redis  *redis.Client
	limit  int
	window time.Duration
}

// NewRateLimiter returns nil when redis is unavailable, and a nil
// limiter's middleware is a pass-through — losing Redis must not take
// the whole API offline.
func NewRateLimiter(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	if client == nil {
		return nil
	}
	return &RateLimiter{redis: client, limit: limit, window: window}
}

// exemptPrefixes never get rate limited.
//
// Webhook endpoints are the important entry: Stripe and Resend retry
// on a 429, but a burst of legitimate deliveries from one provider IP
// looks exactly like abuse, and dropping a payment webhook is far
// worse than serving it. They are authenticated by signature instead.
var exemptPrefixes = []string{"/webhooks/", "/health"}

func isExempt(path string) bool {
	for _, prefix := range exemptPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// RateLimit caps requests per client IP.
func (rl *RateLimiter) RateLimit(next http.Handler) http.Handler {
	if rl == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ip := telemetry.ClientIP(r)
		count, err := rl.incr(r, "rate_limit:ip:"+ip)
		if err != nil {
			// Fail open. A limiter that blocks traffic when its own
			// backing store is down converts a Redis outage into a
			// site outage.
			telemetry.Log.Warn("rate limiter unavailable, allowing request: " + err.Error())
			next.ServeHTTP(w, r)
			return
		}

		remaining := int64(rl.limit) - count
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset",
			fmt.Sprintf("%d", time.Now().Add(rl.window).Unix()))

		if count > int64(rl.limit) {
			telemetry.RateLimitedTotal.WithLabelValues("ip").Inc()
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprintf(w, `{"error":"rate limit exceeded","retry_after":%d}`,
				int(rl.window.Seconds()))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// TenantRateLimit caps requests per authenticated tenant, on top of
// the per-IP limit. One tenant's runaway script should not consume the
// capacity of every other tenant sharing the deployment.
func (rl *RateLimiter) TenantRateLimit(next http.Handler) http.Handler {
	if rl == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := GetTenantID(r.Context())
		if tenantID == "" {
			next.ServeHTTP(w, r)
			return
		}

		count, err := rl.incr(r, "rate_limit:tenant:"+tenantID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		if count > int64(rl.limit) {
			telemetry.RateLimitedTotal.WithLabelValues("tenant").Inc()
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"tenant rate limit exceeded"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// incr bumps the counter and sets the TTL only on the first request of
// a window. Calling EXPIRE on every request would slide the window
// forward indefinitely and the counter would never reset.
func (rl *RateLimiter) incr(r *http.Request, key string) (int64, error) {
	ctx := r.Context()

	count, err := rl.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := rl.redis.Expire(ctx, key, rl.window).Err(); err != nil {
			return count, err
		}
	}
	return count, nil
}
