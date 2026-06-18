package output

import "fmt"

// Transform applies global formatting options like Units and Flatten
// to the API response before it is serialized.
func Transform(value any, opts Options) any {
	if opts.Units == "" {
		opts.Units = "metric"
	}
	m, ok := value.(map[string]any)
	if !ok {
		return value
	}

	// Handle rollup data points
	if dps, ok := m["rollupDataPoints"].([]any); ok && opts.Flatten {
		for i, dp := range dps {
			if dpMap, ok := dp.(map[string]any); ok {
				m["rollupDataPoints"].([]any)[i] = flattenRollupPoint(dpMap, opts)
			}
		}
	}

	// Handle list data points
	if dps, ok := m["dataPoints"].([]any); ok {
		for i, dp := range dps {
			if dpMap, ok := dp.(map[string]any); ok {
				m["dataPoints"].([]any)[i] = transformDataPoint(dpMap, opts)
			}
		}
	}

	return m
}

func flattenRollupPoint(dp map[string]any, opts Options) map[string]any {
	result := make(map[string]any)

	// Extract dates
	if start, ok := dp["civilStartTime"].(map[string]any); ok {
		if d, ok := start["date"].(map[string]any); ok {
			result["date"] = fmt.Sprintf("%04d-%02d-%02d", intVal(d["year"]), intVal(d["month"]), intVal(d["day"]))
		}
	}

	// Flatten Active Zone Minutes
	if azm, ok := dp["activeZoneMinutes"].(map[string]any); ok {
		total := intVal(azm["sumInFatBurnHeartZone"]) + intVal(azm["sumInCardioHeartZone"]) + intVal(azm["sumInPeakHeartZone"])
		result["activeZoneMinutesTotal"] = total
	}

	// Flatten Steps
	if steps, ok := dp["steps"].(map[string]any); ok {
		result["steps"] = intVal(steps["countSum"])
	}

	// Flatten Distance
	if dist, ok := dp["distance"].(map[string]any); ok {
		mm := intVal(dist["millimetersSum"])
		result["distanceKm"] = float64(mm) / 1_000_000
		if opts.Units == "imperial" {
			result["distanceMiles"] = float64(mm) * 0.000000621371
		}
	}

	// Flatten Weight
	if wt, ok := dp["weight"].(map[string]any); ok {
		if _, exists := wt["weightGramsAvg"]; exists {
			g := floatVal(wt["weightGramsAvg"])
			result["weightKg"] = g / 1000
			if opts.Units == "imperial" {
				result["weightLbs"] = g * 0.00220462
			}
		}
	}

	return result
}

func transformDataPoint(dp map[string]any, opts Options) map[string]any {
	if opts.Units == "imperial" {
		// Convert weight
		if weight, ok := dp["weight"].(map[string]any); ok {
			if _, exists := weight["weightGrams"]; exists {
				weight["weightLbs"] = floatVal(weight["weightGrams"]) * 0.00220462
			}
		}
		// Convert distance
		if dist, ok := dp["distance"].(map[string]any); ok {
			if mm, ok := dist["distanceMillimeters"].(float64); ok {
				dist["distanceMiles"] = mm * 0.000000621371
			}
		}
	}

	if opts.Flatten {
		// Flatten Exercise AZM string to int
		if ex, ok := dp["exercise"].(map[string]any); ok {
			if ms, ok := ex["metricsSummary"].(map[string]any); ok {
				if azm, ok := ms["activeZoneMinutes"].(string); ok {
					ms["activeZoneMinutes"] = parseInt(azm)
				}
			}
		}
	}

	return dp
}

func intVal(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		var i int
		fmt.Sscanf(n, "%d", &i)
		return i
	}
	return 0
}

func floatVal(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return f
	}
	return 0
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
