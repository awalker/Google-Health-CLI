// Package clientfilter provides client-side date filtering for Google Health
// data types that do not support server-side date filters (e.g. exercise,
// sleep, daily-resting-heart-rate).
package clientfilter

import (
	"fmt"
	"strings"
	"time"
)

// ParseBound parses a --from or --to flag value into a UTC time.Time.
// Accepts RFC3339 with or without Z suffix, and bare date strings (YYYY-MM-DD).
// The zero value is returned when s is empty (meaning "no bound").
func ParseBound(s string) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, nil
	}
	// Try RFC3339 with Z suffix first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	// Try without Z suffix (civil time format used by some types).
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UTC(), nil
	}
	// Try bare date.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse date/time %q; accepted formats: 2006-01-02, 2006-01-02T15:04:05, 2006-01-02T15:04:05Z", s)
}

// ExtractTime reads a timestamp from a data point using a dot-notation path.
// Two timestamp shapes are supported:
//
//   - RFC3339 string:  "2026-05-29T13:33:06Z"
//   - Civil date map:  {"year":2026,"month":5,"day":29}
//
// Returns zero time and an error if the path is missing or unrecognisable.
func ExtractTime(dataPoint map[string]any, path string) (time.Time, error) {
	val, err := dotGet(dataPoint, strings.Split(path, "."))
	if err != nil {
		return time.Time{}, fmt.Errorf("path %q: %w", path, err)
	}
	switch v := val.(type) {
	case string:
		// Try RFC3339 first, then without Z.
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC(), nil
		}
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t.UTC(), nil
		}
		if t, err := time.Parse("2006-01-02T15:04:05", v); err == nil {
			return t.UTC(), nil
		}
		return time.Time{}, fmt.Errorf("path %q: cannot parse timestamp %q", path, v)
	case map[string]any:
		// {year, month, day} civil date object.
		year, _ := intFromAny(v["year"])
		month, _ := intFromAny(v["month"])
		day, _ := intFromAny(v["day"])
		if year == 0 || month == 0 || day == 0 {
			return time.Time{}, fmt.Errorf("path %q: incomplete date object %v", path, v)
		}
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
	default:
		return time.Time{}, fmt.Errorf("path %q: unexpected type %T", path, val)
	}
}

// FilterDataPoints returns only those data points whose timestamp (at
// clientTimePath) falls within [from, to). A zero from/to means unbounded on
// that side. Data points whose timestamp cannot be extracted are excluded and
// counted in the skipped return value so callers can warn if needed.
func FilterDataPoints(
	dataPoints []any,
	clientTimePath string,
	from, to time.Time,
) (filtered []any, skipped int) {
	for _, dp := range dataPoints {
		point, ok := dp.(map[string]any)
		if !ok {
			skipped++
			continue
		}
		t, err := ExtractTime(point, clientTimePath)
		if err != nil {
			skipped++
			continue
		}
		if !from.IsZero() && t.Before(from) {
			continue
		}
		if !to.IsZero() && !t.Before(to) {
			continue
		}
		filtered = append(filtered, dp)
	}
	return filtered, skipped
}

// dotGet traverses a nested map[string]any using a slice of key segments.
func dotGet(node any, keys []string) (any, error) {
	if len(keys) == 0 {
		return node, nil
	}
	m, ok := node.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object at %q, got %T", keys[0], node)
	}
	val, ok := m[keys[0]]
	if !ok {
		return nil, fmt.Errorf("key %q not found", keys[0])
	}
	return dotGet(val, keys[1:])
}

// intFromAny converts float64 (JSON number default) or int to int.
func intFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}
