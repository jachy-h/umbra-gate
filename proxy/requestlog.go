package proxy

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jachy-h/umbra-gate/db"
)

// maxLoggedBodyBytes caps the size of any single request or response body we
// persist into the request_logs table. Bodies are truncated rather than dropped
// so users can still inspect the prefix when debugging.
const maxLoggedBodyBytes = 64 * 1024

// sensitiveHeaders are redacted before being persisted so we never leak
// credentials into the local debug log.
var sensitiveHeaders = map[string]struct{}{
	"Authorization":       {},
	"X-Api-Key":           {},
	"Cookie":              {},
	"Set-Cookie":          {},
	"Proxy-Authorization": {},
}

// captureRequestLog persists the inbound/outbound HTTP exchange for a session,
// truncating oversized bodies and redacting sensitive headers. Errors are
// swallowed (only logged) so logging never breaks the proxy path.
func captureRequestLog(database *db.DB, log db.RequestLog) {
	if database == nil {
		return
	}
	log.RequestBody = truncateBody(log.RequestBody)
	log.ResponseBody = truncateBody(log.ResponseBody)
	if _, err := database.InsertRequestLog(log); err != nil {
		slog.Error("failed to persist request log", "error", err, "session_id", log.SessionID)
	}
}

// serializeHeaders renders headers as a single string with one "Key: value"
// pair per line, redacting sensitive credentials.
func serializeHeaders(h http.Header) string {
	if len(h) == 0 {
		return ""
	}
	var b strings.Builder
	for key, values := range h {
		canon := http.CanonicalHeaderKey(key)
		if _, redact := sensitiveHeaders[canon]; redact {
			b.WriteString(canon)
			b.WriteString(": [REDACTED]\n")
			continue
		}
		for _, v := range values {
			b.WriteString(canon)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func truncateBody(body string) string {
	if len(body) <= maxLoggedBodyBytes {
		return body
	}
	return body[:maxLoggedBodyBytes] + "\n…[truncated]"
}
