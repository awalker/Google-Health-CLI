package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rudrankriyam/Google-Health-CLI/internal/healthapi"
	"github.com/rudrankriyam/Google-Health-CLI/internal/registry"
)

func TestCivilRangeSameDateProducesEndGreaterThanStart(t *testing.T) {
	result, err := civilRange("2025-06-13", "2025-06-13")
	if err != nil {
		t.Fatalf("civilRange failed: %v", err)
	}

	startDate := result["start"].(map[string]any)["date"].(map[string]any)
	endDate := result["end"].(map[string]any)["date"].(map[string]any)

	toInt := func(v any) int {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
		return 0
	}

	startTime := time.Date(
		toInt(startDate["year"]),
		time.Month(toInt(startDate["month"])),
		toInt(startDate["day"]),
		0, 0, 0, 0, time.UTC,
	)
	endTime := time.Date(
		toInt(endDate["year"]),
		time.Month(toInt(endDate["month"])),
		toInt(endDate["day"]),
		0, 0, 0, 0, time.UTC,
	)

	if !endTime.After(startTime) {
		t.Errorf("end time %v should be strictly after start time %v", endTime, startTime)
	}
}

func TestCivilRangeDifferentDates(t *testing.T) {
	result, err := civilRange("2025-06-01", "2025-06-07")
	if err != nil {
		t.Fatalf("civilRange failed: %v", err)
	}

	startDate := result["start"].(map[string]any)["date"].(map[string]any)
	endDate := result["end"].(map[string]any)["date"].(map[string]any)

	toInt := func(v any) int {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
		return 0
	}

	if toInt(startDate["day"]) != 1 {
		t.Errorf("start day = %v, want 1", startDate["day"])
	}
	if toInt(endDate["day"]) != 7 {
		t.Errorf("end day = %v, want 7", endDate["day"])
	}
}

func TestFetchAndTransformUsesRollupWhenRequested(t *testing.T) {
	rollupCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v4/users/me/dataTypes/steps/dataPoints:dailyRollUp" {
			rollupCalled = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rollUpDataPoints":[]}`))
	}))
	defer srv.Close()

	client := healthapi.New(srv.URL, "users/me", srv.Client())
	dt, _ := registry.Lookup("steps")

	_, err := fetchAndTransform(context.Background(), client, dt, "2025-06-01", "2025-06-07", false, "", true, 0)
	if err != nil {
		t.Fatalf("fetchAndTransform failed: %v", err)
	}

	if !rollupCalled {
		t.Error("expected DailyRollUp endpoint to be called when rollup=true")
	}
}

func TestFetchAndTransformUsesListWhenRollupFalse(t *testing.T) {
	listCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v4/users/me/dataTypes/steps/dataPoints" {
			listCalled = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"dataPoints":[]}`))
	}))
	defer srv.Close()

	client := healthapi.New(srv.URL, "users/me", srv.Client())
	dt, _ := registry.Lookup("steps")

	_, err := fetchAndTransform(context.Background(), client, dt, "2025-06-01", "2025-06-07", false, "", false, 0)
	if err != nil {
		t.Fatalf("fetchAndTransform failed: %v", err)
	}

	if !listCalled {
		t.Error("expected ListDataPoints endpoint to be called when rollup=false")
	}
}

func TestFetchAndTransformSingleDateRollup(t *testing.T) {
	rollupCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v4/users/me/dataTypes/steps/dataPoints:dailyRollUp" {
			rollupCalled = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rollUpDataPoints":[]}`))
	}))
	defer srv.Close()

	client := healthapi.New(srv.URL, "users/me", srv.Client())
	dt, _ := registry.Lookup("steps")

	_, err := fetchAndTransform(context.Background(), client, dt, "2025-06-13", "2025-06-13", false, "", false, 0)
	if err != nil {
		t.Fatalf("fetchAndTransform failed: %v", err)
	}

	if !rollupCalled {
		t.Error("expected DailyRollUp endpoint to be called for single date query")
	}
}

func TestToolDefinitionsHaveNewParams(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(true))
	buildTools(s)

	tools := s.ListTools()
	getHealthDataTool, ok := tools["get_health_data"]
	if !ok {
		t.Fatal("get_health_data tool not found")
	}

	paramNames := extractParamNames(&getHealthDataTool.Tool)
	requiredParams := []string{"rollup", "page_token", "page_size"}
	for _, name := range requiredParams {
		found := false
		for _, p := range paramNames {
			if p == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("get_health_data missing parameter: %s", name)
		}
	}
}

func TestPaginationReturnsNextPageToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"dataPoints":[{"id":"p1"}], "nextPageToken":"abc123"}`))
	}))
	defer srv.Close()

	client := healthapi.New(srv.URL, "users/me", srv.Client())

	result, err := client.ListDataPoints(context.Background(), "steps", healthapi.ListOptions{PageToken: "abc123"})
	if err != nil {
		t.Fatalf("ListDataPoints failed: %v", err)
	}

	token, ok := result["nextPageToken"].(string)
	if !ok || token != "abc123" {
		t.Errorf("nextPageToken = %v, want 'abc123'", token)
	}
}

func TestPaginationWithFilterForFilterableTypes(t *testing.T) {
	var receivedFilter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedFilter = r.URL.Query().Get("filter")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"dataPoints":[{"id":"p2"}], "nextPageToken":""}`))
	}))
	defer srv.Close()

	client := healthapi.New(srv.URL, "users/me", srv.Client())
	dt, _ := registry.Lookup("steps")

	listOpts := healthapi.ListOptions{PageToken: "token123", PageSize: 500}
	if dt.Filterable {
		listOpts.Filter = registry.FilterFromRange(dt, "2025-06-01", "2025-06-08")
	}

	_, err := client.ListDataPoints(context.Background(), dt.EndpointName, listOpts)
	if err != nil {
		t.Fatalf("ListDataPoints failed: %v", err)
	}

	if receivedFilter == "" {
		t.Error("expected filter to be passed with pageToken for filterable type")
	}
}

func TestDailySummaryUsesRollupForEligibleTypes(t *testing.T) {
	rollupCalls := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v4/users/me/dataTypes/steps/dataPoints:dailyRollUp" {
			rollupCalls["steps"]++
		}
		if r.URL.Path == "/v4/users/me/dataTypes/active-zone-minutes/dataPoints:dailyRollUp" {
			rollupCalls["active-zone-minutes"]++
		}
		if r.URL.Path == "/v4/users/me/dataTypes/distance/dataPoints:dailyRollUp" {
			rollupCalls["distance"]++
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rollUpDataPoints":[]}`))
	}))
	defer srv.Close()

	client := healthapi.New(srv.URL, "users/me", srv.Client())
	date := "2025-06-13"

	rollupTypes := []string{"steps", "active-zone-minutes", "distance"}
	for _, typeName := range rollupTypes {
		dt, ok := registry.Lookup(typeName)
		if !ok {
			t.Fatalf("Lookup(%q) failed", typeName)
		}
		useRollup := registry.HasOperation(dt, "dailyRollUp")
		_, err := fetchAndTransform(context.Background(), client, dt, date, date, true, "", useRollup, 0)
		if err != nil {
			t.Fatalf("fetchAndTransform(%s) failed: %v", typeName, err)
		}
	}

	for _, name := range rollupTypes {
		if rollupCalls[name] == 0 {
			t.Errorf("expected rollup call for %s", name)
		}
	}
}

func buildTools(s *server.MCPServer) {
	getCapabilitiesTool := mcp.NewTool("get_health_capabilities",
		mcp.WithDescription("Returns a list of all 31 Google Health data types and their supported operations."),
	)
	s.AddTool(getCapabilitiesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("test"), nil
	})

	getHealthDataTool := mcp.NewTool("get_health_data",
		mcp.WithDescription("Fetches health data for a specific type."),
		mcp.WithString("data_type",
			mcp.Required(),
			mcp.Description("The Google Health data type."),
		),
		mcp.WithString("from",
			mcp.Description("Start date (YYYY-MM-DD) or datetime."),
		),
		mcp.WithString("to",
			mcp.Description("End date (YYYY-MM-DD) or datetime."),
		),
		mcp.WithBoolean("flatten",
			mcp.Description("If true, returns a simplified, flat structure."),
		),
		mcp.WithString("units",
			mcp.Description("Measurement units: 'metric' (default) or 'imperial'."),
		),
		mcp.WithBoolean("rollup",
			mcp.Description("If true, uses the dailyRollUp endpoint."),
		),
		mcp.WithString("page_token",
			mcp.Description("Pagination token from a previous response."),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Number of results per page."),
		),
	)
	s.AddTool(getHealthDataTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("test"), nil
	})
}

func extractParamNames(tool *mcp.Tool) []string {
	var names []string
	data, _ := json.Marshal(tool)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if inputSchema, ok := raw["inputSchema"].(map[string]any); ok {
		if properties, ok := inputSchema["properties"].(map[string]any); ok {
			for name := range properties {
				names = append(names, name)
			}
		}
	}
	return names
}

func TestExtractSleepSummaryStages(t *testing.T) {
	response := map[string]any{
		"dataPoints": []any{
			map[string]any{
				"sleep": map[string]any{
					"type": "STAGES",
					"summary": map[string]any{
						"minutesAsleep": "435",
						"minutesAwake":  "5",
						"stagesSummary": []any{
							map[string]any{"type": "AWAKE", "minutes": "5", "count": "1"},
							map[string]any{"type": "LIGHT", "minutes": "201", "count": "13"},
							map[string]any{"type": "DEEP", "minutes": "96", "count": "6"},
							map[string]any{"type": "REM", "minutes": "137", "count": "7"},
						},
					},
				},
			},
		},
	}

	result := extractSleepSummary(response)
	if result == nil {
		t.Fatal("expected non-nil sleep summary")
	}
	if got, want := result["minutesAsleep"], 435; got != want {
		t.Errorf("minutesAsleep = %v, want %v", got, want)
	}
	if got, want := result["deepMinutes"], 96; got != want {
		t.Errorf("deepMinutes = %v, want %v", got, want)
	}
	if got, want := result["remMinutes"], 137; got != want {
		t.Errorf("remMinutes = %v, want %v", got, want)
	}
	if got, want := result["lightMinutes"], 201; got != want {
		t.Errorf("lightMinutes = %v, want %v", got, want)
	}
}

func TestExtractSleepSummaryPrefersStages(t *testing.T) {
	response := map[string]any{
		"dataPoints": []any{
			map[string]any{
				"sleep": map[string]any{
					"type": "CLASSIC",
					"summary": map[string]any{
						"minutesAsleep": "400",
					},
				},
			},
			map[string]any{
				"sleep": map[string]any{
					"type": "STAGES",
					"summary": map[string]any{
						"minutesAsleep": "435",
						"stagesSummary": []any{
							map[string]any{"type": "DEEP", "minutes": "96"},
						},
					},
				},
			},
		},
	}

	result := extractSleepSummary(response)
	if result == nil {
		t.Fatal("expected non-nil sleep summary")
	}
	if got, want := result["minutesAsleep"], 435; got != want {
		t.Errorf("should prefer STAGES sleep: minutesAsleep = %v, want %v", got, want)
	}
}

func TestExtractSleepSummaryEmpty(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
	}{
		{"no dataPoints", map[string]any{"dataPoints": []any{}}},
		{"no sleep field", map[string]any{"dataPoints": []any{map[string]any{"other": true}}}},
		{"no summary", map[string]any{"dataPoints": []any{map[string]any{"sleep": map[string]any{"type": "STAGES"}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSleepSummary(tt.response); got != nil {
				t.Errorf("expected nil for %q, got %v", tt.name, got)
			}
		})
	}
}

func TestExtractExerciseSummary(t *testing.T) {
	response := map[string]any{
		"dataPoints": []any{
			map[string]any{
				"exercise": map[string]any{
					"exerciseType": "BIKING",
					"metricsSummary": map[string]any{
						"activeZoneMinutes": "14",
					},
				},
			},
			map[string]any{
				"exercise": map[string]any{
					"exerciseType": "WALKING",
					"metricsSummary": map[string]any{
						"activeZoneMinutes": "19",
					},
				},
			},
		},
	}

	result := extractExerciseSummary(response)
	if result == nil {
		t.Fatal("expected non-nil exercise summary")
	}
	if got, want := result["workoutCount"], 2; got != want {
		t.Errorf("workoutCount = %v, want %v", got, want)
	}
	types, _ := result["workoutTypes"].([]string)
	if len(types) != 2 {
		t.Errorf("workoutTypes length = %d, want 2", len(types))
	}
	if got, want := result["totalActiveMinutes"], 33; got != want {
		t.Errorf("totalActiveMinutes = %v, want %v", got, want)
	}
}

func TestExtractExerciseSummaryDeduplicatesTypes(t *testing.T) {
	response := map[string]any{
		"dataPoints": []any{
			map[string]any{
				"exercise": map[string]any{
					"exerciseType": "BIKING",
					"metricsSummary": map[string]any{
						"activeZoneMinutes": "10",
					},
				},
			},
			map[string]any{
				"exercise": map[string]any{
					"exerciseType": "BIKING",
					"metricsSummary": map[string]any{
						"activeZoneMinutes": "20",
					},
				},
			},
		},
	}

	result := extractExerciseSummary(response)
	if result == nil {
		t.Fatal("expected non-nil exercise summary")
	}
	types, _ := result["workoutTypes"].([]string)
	if len(types) != 1 {
		t.Errorf("workoutTypes length = %d, want 1", len(types))
	}
	if got, want := result["totalActiveMinutes"], 30; got != want {
		t.Errorf("totalActiveMinutes = %v, want %v", got, want)
	}
}

func TestExtractExerciseSummaryEmpty(t *testing.T) {
	if got := extractExerciseSummary(map[string]any{"dataPoints": []any{}}); got != nil {
		t.Errorf("expected nil for empty dataPoints, got %v", got)
	}
	if got := extractExerciseSummary(map[string]any{"dataPoints": []any{map[string]any{"other": true}}}); got != nil {
		t.Errorf("expected nil for no exercise field, got %v", got)
	}
}

func TestDailySummaryReturnsSleepAndExercise(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "sleep"):
			w.Write([]byte(`{"dataPoints":[{"sleep":{"interval":{"startTime":"2025-06-13T07:00:00Z"},"type":"STAGES","summary":{"minutesAsleep":"435","stagesSummary":[{"type":"DEEP","minutes":"96"},{"type":"REM","minutes":"137"},{"type":"LIGHT","minutes":"201"}]}}}]}`))
		case strings.Contains(r.URL.Path, "exercise"):
			w.Write([]byte(`{"dataPoints":[{"exercise":{"interval":{"startTime":"2025-06-13T14:00:00Z"},"exerciseType":"WALKING","metricsSummary":{"activeZoneMinutes":"19"}}}]}`))
		default:
			w.Write([]byte(`{"rollUpDataPoints":[]}`))
		}
	}))
	defer srv.Close()

	client := healthapi.New(srv.URL, "users/me", srv.Client())

	sleepSummary := extractDailySummarySleep(t, client, "sleep")
	if sleepSummary == nil {
		t.Fatal("extracted sleep summary is nil")
	}
	if got, want := sleepSummary["minutesAsleep"], 435; got != want {
		t.Errorf("minutesAsleep = %v, want %v", got, want)
	}

	exerciseSummary := extractDailySummaryExercise(t, client, "exercise")
	if exerciseSummary == nil {
		t.Fatal("extracted exercise summary is nil")
	}
	if got, want := exerciseSummary["workoutCount"], 1; got != want {
		t.Errorf("workoutCount = %v, want %v", got, want)
	}
}

func extractDailySummarySleep(t *testing.T, client *healthapi.Client, typeName string) map[string]any {
	t.Helper()
	dt, _ := registry.Lookup(typeName)
	result, err := fetchAndTransform(context.Background(), client, dt, "2025-06-13", "2025-06-13", true, "", false, 0)
	if err != nil {
		t.Fatalf("fetchAndTransform(%s) failed: %v", typeName, err)
	}
	return extractSleepSummary(result)
}

func extractDailySummaryExercise(t *testing.T, client *healthapi.Client, typeName string) map[string]any {
	t.Helper()
	dt, _ := registry.Lookup(typeName)
	result, err := fetchAndTransform(context.Background(), client, dt, "2025-06-13", "2025-06-13", true, "", false, 0)
	if err != nil {
		t.Fatalf("fetchAndTransform(%s) failed: %v", typeName, err)
	}
	return extractExerciseSummary(result)
}
