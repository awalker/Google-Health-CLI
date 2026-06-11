package clientfilter

import (
	"testing"
	"time"
)

func TestParseBound(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		want    time.Time
	}{
		{"", false, time.Time{}},
		{"2026-05-01", false, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-05-01T00:00:00Z", false, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-05-01T00:00:00", false, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"not-a-date", true, time.Time{}},
	}
	for _, tc := range tests {
		got, err := ParseBound(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseBound(%q) expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseBound(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if !got.Equal(tc.want) {
			t.Errorf("ParseBound(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestExtractTimeRFC3339(t *testing.T) {
	point := map[string]any{
		"exercise": map[string]any{
			"interval": map[string]any{
				"startTime": "2026-05-29T13:33:06Z",
			},
		},
	}
	got, err := ExtractTime(point, "exercise.interval.startTime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 29, 13, 33, 6, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractTime = %v, want %v", got, want)
	}
}

func TestExtractTimeCivilDate(t *testing.T) {
	point := map[string]any{
		"dailyRestingHeartRate": map[string]any{
			"date": map[string]any{
				"year":  float64(2026),
				"month": float64(6),
				"day":   float64(10),
			},
		},
	}
	got, err := ExtractTime(point, "dailyRestingHeartRate.date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractTime = %v, want %v", got, want)
	}
}

func TestExtractTimeMissingPath(t *testing.T) {
	point := map[string]any{"exercise": map[string]any{}}
	_, err := ExtractTime(point, "exercise.interval.startTime")
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
}

func TestFilterDataPoints(t *testing.T) {
	makePoint := func(ts string) any {
		return map[string]any{
			"exercise": map[string]any{
				"interval": map[string]any{"startTime": ts},
			},
		}
	}

	points := []any{
		makePoint("2026-04-30T10:00:00Z"), // before range
		makePoint("2026-05-01T00:00:00Z"), // == from (inclusive)
		makePoint("2026-05-15T12:00:00Z"), // inside
		makePoint("2026-06-01T00:00:00Z"), // == to (exclusive)
		makePoint("2026-06-02T00:00:00Z"), // after range
	}

	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	got, skipped := FilterDataPoints(points, "exercise.interval.startTime", from, to)
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	if len(got) != 2 {
		t.Errorf("len(filtered) = %d, want 2", len(got))
	}
}

func TestFilterDataPointsNoBounds(t *testing.T) {
	makePoint := func(ts string) any {
		return map[string]any{
			"sleep": map[string]any{
				"interval": map[string]any{"startTime": ts},
			},
		}
	}
	points := []any{
		makePoint("2026-04-01T00:00:00Z"),
		makePoint("2026-05-01T00:00:00Z"),
	}
	got, _ := FilterDataPoints(points, "sleep.interval.startTime", time.Time{}, time.Time{})
	if len(got) != 2 {
		t.Errorf("with no bounds, all points should pass; got %d", len(got))
	}
}

func TestFilterDataPointsSkipsBadPoints(t *testing.T) {
	points := []any{
		"not-a-map",      // wrong type
		map[string]any{}, // missing path
		map[string]any{
			"exercise": map[string]any{
				"interval": map[string]any{"startTime": "2026-05-10T00:00:00Z"},
			},
		},
	}
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	got, skipped := FilterDataPoints(points, "exercise.interval.startTime", from, time.Time{})
	if skipped != 2 {
		t.Errorf("skipped = %d, want 2", skipped)
	}
	if len(got) != 1 {
		t.Errorf("len(filtered) = %d, want 1", len(got))
	}
}
