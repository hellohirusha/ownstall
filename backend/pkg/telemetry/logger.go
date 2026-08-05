// Package telemetry holds the observability plumbing: structured
// logging, Prometheus metrics, distributed tracing and error
// reporting. Every piece degrades to a no-op when its backend is not
// configured, so the API runs identically on a laptop with none of it.
package telemetry

import (
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log is the process-wide structured logger. It is never nil: until
// InitLogger runs it is a no-op, so an early log line cannot panic.
var Log = zap.NewNop()

// InitLogger builds a logger suited to the environment: JSON in
// production so Railway's log drain can parse it, human-readable
// elsewhere.
func InitLogger() error {
	var config zap.Config

	if os.Getenv("ENVIRONMENT") == "production" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return err
	}

	Log = logger
	zap.ReplaceGlobals(logger)
	return nil
}

// SyncLogger flushes buffered log entries at shutdown.
func SyncLogger() {
	// Sync on a terminal returns an error on some platforms with
	// nothing to fix, so the result is deliberately dropped.
	_ = Log.Sync()
}

// RequestLogger logs one structured line per request, at a level that
// matches the outcome so 5xx stands out in a log search.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := wrapWriter(w)

		next.ServeHTTP(wrapped, r)

		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", wrapped.statusCode),
			zap.Duration("duration", time.Since(start)),
			zap.String("ip", ClientIP(r)),
			zap.String("user_agent", r.UserAgent()),
		}

		// Correlates this line with the trace and with chi's request id
		if id := r.Header.Get("X-Request-ID"); id != "" {
			fields = append(fields, zap.String("request_id", id))
		}
		if traceID := TraceIDFromContext(r.Context()); traceID != "" {
			fields = append(fields, zap.String("trace_id", traceID))
		}

		switch {
		case wrapped.statusCode >= 500:
			Log.Error("http request", fields...)
		case wrapped.statusCode >= 400:
			Log.Warn("http request", fields...)
		default:
			Log.Info("http request", fields...)
		}
	})
}
