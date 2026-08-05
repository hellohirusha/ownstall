package telemetry

import "strings"

// splitAndTrim splits on sep and drops empty fields.
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// cut splits s around the first instance of sep.
func cut(s, sep string) (before, after string, found bool) {
	before, after, found = strings.Cut(s, sep)
	return strings.TrimSpace(before), strings.TrimSpace(after), found
}
