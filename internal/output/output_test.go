package output

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Print(&buf, map[string]any{"ok": true}, Options{Format: FormatJSON, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatalf("JSON output = %q", buf.String())
	}
}

func TestPrintNDJSONUsesRows(t *testing.T) {
	var buf bytes.Buffer
	err := Print(&buf, []map[string]any{{"name": "steps"}, {"name": "sleep"}}, Options{Format: FormatNDJSON})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("NDJSON lines = %d, want 2: %q", len(lines), buf.String())
	}
}

func TestFlattenRollupPointHandlesWeight(t *testing.T) {
	dp := map[string]any{
		"civilStartTime": map[string]any{
			"date": map[string]any{
				"year": float64(2026), "month": float64(6), "day": float64(18),
			},
		},
		"weight": map[string]any{
			"weightGramsAvg": float64(91943),
		},
		"steps": map[string]any{
			"countSum": float64(8500),
		},
	}

	t.Run("metric", func(t *testing.T) {
		result := flattenRollupPoint(dp, Options{Units: "metric"})

		if got, want := result["date"], "2026-06-18"; got != want {
			t.Fatalf("date = %v, want %v", got, want)
		}
		gotKg, ok := result["weightKg"].(float64)
		if !ok {
			t.Fatalf("weightKg missing or wrong type: %T", result["weightKg"])
		}
		wantKg := 91943.0 / 1000
		if math.Abs(gotKg-wantKg) > 0.001 {
			t.Fatalf("weightKg = %v, want %v", gotKg, wantKg)
		}
		if _, exists := result["weightLbs"]; exists {
			t.Fatal("weightLbs should not be present in metric mode")
		}
		if got, want := result["steps"], 8500; got != want {
			t.Fatalf("steps = %v, want %v", got, want)
		}
	})

	t.Run("imperial", func(t *testing.T) {
		result := flattenRollupPoint(dp, Options{Units: "imperial"})

		gotKg, ok := result["weightKg"].(float64)
		if !ok {
			t.Fatalf("weightKg missing or wrong type: %T", result["weightKg"])
		}
		wantKg := 91943.0 / 1000
		if math.Abs(gotKg-wantKg) > 0.001 {
			t.Fatalf("weightKg = %v, want %v", gotKg, wantKg)
		}
		gotLbs, ok := result["weightLbs"].(float64)
		if !ok {
			t.Fatalf("weightLbs missing or wrong type: %T", result["weightLbs"])
		}
		wantLbs := 91943.0 * 0.00220462
		if math.Abs(gotLbs-wantLbs) > 0.01 {
			t.Fatalf("weightLbs = %v, want %v", gotLbs, wantLbs)
		}
	})

	t.Run("no weight field", func(t *testing.T) {
		dpNoWeight := map[string]any{
			"civilStartTime": map[string]any{
				"date": map[string]any{
					"year": float64(2026), "month": float64(6), "day": float64(18),
				},
			},
		}
		result := flattenRollupPoint(dpNoWeight, Options{Units: "metric"})
		if _, exists := result["weightKg"]; exists {
			t.Fatal("weightKg should not be present when weight field is missing")
		}
		if _, exists := result["weightLbs"]; exists {
			t.Fatal("weightLbs should not be present when weight field is missing")
		}
	})

	t.Run("weightGramsAvg as int", func(t *testing.T) {
		dpInt := map[string]any{
			"civilStartTime": map[string]any{
				"date": map[string]any{
					"year": float64(2026), "month": float64(6), "day": float64(18),
				},
			},
			"weight": map[string]any{
				"weightGramsAvg": 91943,
			},
		}
		result := flattenRollupPoint(dpInt, Options{Units: "metric"})
		gotKg, ok := result["weightKg"].(float64)
		if !ok {
			t.Fatalf("weightKg missing or wrong type: %T", result["weightKg"])
		}
		wantKg := 91943.0 / 1000
		if math.Abs(gotKg-wantKg) > 0.001 {
			t.Fatalf("weightKg = %v, want %v", gotKg, wantKg)
		}
	})
}

func TestTransformRollupPreservesWeight(t *testing.T) {
	response := map[string]any{
		"rollupDataPoints": []any{
			map[string]any{
				"civilStartTime": map[string]any{
					"date": map[string]any{
						"year": float64(2026), "month": float64(6), "day": float64(18),
					},
				},
				"weight": map[string]any{
					"weightGramsAvg": float64(91943),
				},
				"steps": map[string]any{
					"countSum": float64(8500),
				},
			},
		},
	}

	result := Transform(response, Options{Units: "metric", Flatten: true})
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %T, want map[string]any", result)
	}
	dps, ok := m["rollupDataPoints"].([]any)
	if !ok {
		t.Fatalf("rollupDataPoints = %T, want []any", m["rollupDataPoints"])
	}
	if len(dps) != 1 {
		t.Fatalf("len(rollupDataPoints) = %d, want 1", len(dps))
	}
	flat, ok := dps[0].(map[string]any)
	if !ok {
		t.Fatalf("flat dp = %T, want map[string]any", dps[0])
	}
	if _, exists := flat["weightKg"]; !exists {
		t.Fatalf("weightKg missing from flattened rollup point: %v", flat)
	}
	if got := flat["steps"]; got != 8500 {
		t.Fatalf("steps = %v, want 8500", got)
	}
}
