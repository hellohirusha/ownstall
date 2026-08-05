package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/telemetry"
)

// AuditLogger records state-changing GraphQL operations.
type AuditLogger struct {
	DB *pgxpool.Pool
}

// maxAuditBodyBytes bounds how much of a request body we buffer to
// identify the operation.
const maxAuditBodyBytes = 64 << 10

// graphQLRequest is the subset of the POST body we need.
type graphQLRequest struct {
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
}

// AuditMutations writes an audit row for every GraphQL mutation.
//
// Queries are deliberately not audited: they change nothing, they are
// the overwhelming majority of traffic, and logging them would bury
// the writes that an audit trail exists to answer questions about.
func (al *AuditLogger) AuditMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxAuditBodyBytes))
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		// The handler still needs the body we just consumed
		r.Body = io.NopCloser(bytes.NewReader(body))

		var parsed graphQLRequest
		if err := json.Unmarshal(body, &parsed); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		operation, isMutation := mutationName(parsed)
		if !isMutation {
			next.ServeHTTP(w, r)
			return
		}

		wrapped := &auditWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		al.record(r, operation, wrapped.statusCode < 400)
	})
}

// mutationName reports the operation name and whether the document is
// a mutation. gqlgen accepts an unnamed operation, so the document
// text is the authority on the operation type and operationName is
// only a label.
func mutationName(req graphQLRequest) (string, bool) {
	query := strings.TrimSpace(req.Query)

	// Skip a leading comment block before checking the keyword
	for strings.HasPrefix(query, "#") {
		_, rest, found := strings.Cut(query, "\n")
		if !found {
			return "", false
		}
		query = strings.TrimSpace(rest)
	}

	if !strings.HasPrefix(query, "mutation") {
		return "", false
	}

	if req.OperationName != "" {
		return req.OperationName, true
	}
	return firstSelection(query), true
}

// firstSelection pulls the field name out of an anonymous mutation
// body so the audit row says more than "mutation".
func firstSelection(query string) string {
	open := strings.Index(query, "{")
	if open < 0 {
		return "mutation"
	}

	rest := strings.TrimSpace(query[open+1:])
	end := strings.IndexAny(rest, "({ \n\t\r")
	if end <= 0 {
		return "mutation"
	}
	return rest[:end]
}

// record writes the audit row without blocking the response. The
// request context is already cancelled by then, so it uses its own.
func (al *AuditLogger) record(r *http.Request, action string, success bool) {
	tenantID := GetTenantID(r.Context())
	userID := GetUserID(r.Context())
	ip := telemetry.ClientIP(r)
	userAgent := r.UserAgent()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := al.DB.Exec(ctx, `
            INSERT INTO audit_logs
                (tenant_id, user_id, action, ip_address, user_agent, success)
            VALUES ($1, $2, $3, $4, $5, $6)
        `, nullable(tenantID), nullable(userID), action, ip, userAgent, success)
		if err != nil {
			telemetry.Log.Warn("failed to write audit log: " + err.Error())
		}
	}()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

type auditWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (aw *auditWriter) WriteHeader(code int) {
	if aw.wroteHeader {
		return
	}
	aw.statusCode = code
	aw.wroteHeader = true
	aw.ResponseWriter.WriteHeader(code)
}

func (aw *auditWriter) Write(b []byte) (int, error) {
	aw.wroteHeader = true
	return aw.ResponseWriter.Write(b)
}
