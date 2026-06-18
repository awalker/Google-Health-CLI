package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/oauth2"

	"github.com/rudrankriyam/Google-Health-CLI/internal/auth"
	"github.com/rudrankriyam/Google-Health-CLI/internal/clientfilter"
	"github.com/rudrankriyam/Google-Health-CLI/internal/config"
	"github.com/rudrankriyam/Google-Health-CLI/internal/healthapi"
	"github.com/rudrankriyam/Google-Health-CLI/internal/output"
	"github.com/rudrankriyam/Google-Health-CLI/internal/registry"
)

const (
	maxPageSize         = 1000
	maxTypesCount       = 20
	maxTypesParamLen    = 500
	dailySummaryTimeout = 60 * time.Second
)

// newHealthClient loads the local config, creates an OAuth token source,
// and returns an authenticated healthapi.Client.
func newHealthClient(ctx context.Context) (*healthapi.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("ghealth-mcp: failed to load config: %v", err)
		return nil, fmt.Errorf("failed to load configuration")
	}

	source, err := auth.TokenSource(ctx, cfg)
	if err != nil {
		log.Printf("ghealth-mcp: authentication error: %v", err)
		return nil, fmt.Errorf("authentication failed")
	}

	return healthapi.New(cfg.BaseURL, cfg.User, oauth2.NewClient(ctx, source)), nil
}

// civilRange builds a daily rollup range body from date-only strings,
// matching the format used by the Google Health API's dailyRollUp endpoint.
func civilRange(from, to string) (map[string]any, error) {
	result := map[string]any{}
	if from != "" {
		start, err := parseCivilDate(from)
		if err != nil {
			return nil, fmt.Errorf("from: %w", err)
		}
		result["start"] = start
	}
	if to != "" {
		end, err := parseCivilDate(to)
		if err != nil {
			return nil, fmt.Errorf("to: %w", err)
		}
		result["end"] = end
	}
	if from == to && from != "" {
		t, err := time.Parse("2006-01-02", from)
		if err != nil {
			return nil, fmt.Errorf("from: %w", err)
		}
		endDate, err := parseCivilDate(t.AddDate(0, 0, 1).Format("2006-01-02"))
		if err != nil {
			return nil, fmt.Errorf("computing end date: %w", err)
		}
		result["end"] = endDate
	}
	return result, nil
}

func parseCivilDate(s string) (map[string]any, error) {
	datePart, _, _ := strings.Cut(s, "T")
	t, err := time.Parse("2006-01-02", datePart)
	if err != nil {
		return nil, fmt.Errorf("expected YYYY-MM-DD: %w", err)
	}
	return map[string]any{
		"date": map[string]any{
			"year":  t.Year(),
			"month": int(t.Month()),
			"day":   t.Day(),
		},
	}, nil
}

func extractSleepSummary(response map[string]any) map[string]any {
	dps, _ := response["dataPoints"].([]any)
	if len(dps) == 0 {
		return nil
	}

	var sleepMap map[string]any
	for _, dp := range dps {
		dpMap, ok := dp.(map[string]any)
		if !ok {
			continue
		}
		sleep, ok := dpMap["sleep"].(map[string]any)
		if !ok {
			continue
		}
		sleepMap = sleep
		if t, _ := sleep["type"].(string); t == "STAGES" {
			break
		}
	}

	if sleepMap == nil {
		return nil
	}

	summary, ok := sleepMap["summary"].(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)
	if v, err := strconv.Atoi(fmt.Sprint(summary["minutesAsleep"])); err == nil {
		result["minutesAsleep"] = v
	}

	if stages, ok := summary["stagesSummary"].([]any); ok {
		for _, s := range stages {
			stage, ok := s.(map[string]any)
			if !ok {
				continue
			}
			stype, _ := stage["type"].(string)
			minutes, _ := strconv.Atoi(fmt.Sprint(stage["minutes"]))
			switch stype {
			case "DEEP":
				result["deepMinutes"] = minutes
			case "REM":
				result["remMinutes"] = minutes
			case "LIGHT":
				result["lightMinutes"] = minutes
			}
		}
	}

	return result
}

func extractExerciseSummary(response map[string]any) map[string]any {
	dps, _ := response["dataPoints"].([]any)
	if len(dps) == 0 {
		return nil
	}

	var types []string
	seen := map[string]bool{}
	totalAZM := 0
	workoutCount := 0

	for _, dp := range dps {
		dpMap, ok := dp.(map[string]any)
		if !ok {
			continue
		}
		ex, ok := dpMap["exercise"].(map[string]any)
		if !ok {
			continue
		}
		workoutCount++

		if exType, ok := ex["exerciseType"].(string); ok && !seen[exType] {
			seen[exType] = true
			types = append(types, exType)
		}

		if ms, ok := ex["metricsSummary"].(map[string]any); ok {
			n, _ := strconv.Atoi(fmt.Sprint(ms["activeZoneMinutes"]))
			totalAZM += n
		}
	}

	if workoutCount == 0 {
		return nil
	}

	return map[string]any{
		"workoutCount":       workoutCount,
		"workoutTypes":       types,
		"totalActiveMinutes": totalAZM,
	}
}

// fetchAndTransform fetches data for a single type and returns the
// transformed response map, or an error message string (empty on success).
func fetchAndTransform(ctx context.Context, client *healthapi.Client, dt registry.DataType, from, to string, flatten bool, units string, rollup bool, pageSize int, pageToken string) (map[string]any, error) {
	hasList := registry.HasOperation(dt, "list")
	hasRollup := registry.HasOperation(dt, "dailyRollUp")

	if !hasList && !hasRollup {
		return nil, fmt.Errorf("data type %s does not support list or rollup", dt.EndpointName)
	}

	singleDateRollup := hasRollup && from != "" && from == to && !strings.Contains(from, "T")
	useRollup := hasRollup && (rollup || singleDateRollup || !hasList)

	if !useRollup {
		if from != "" && from == to && !strings.Contains(from, "T") {
			t, err := time.Parse("2006-01-02", from)
			if err != nil {
				return nil, fmt.Errorf("from: invalid date: %w", err)
			}
			to = t.AddDate(0, 0, 1).Format("2006-01-02")
		}
		if !strings.Contains(from, "T") && strings.Contains(dt.DefaultTimePath, "physical_time") {
			if from != "" {
				from += "T00:00:00Z"
			}
			if to != "" && !strings.Contains(to, "T") {
				to += "T00:00:00Z"
			}
		}
	}

	meta := map[string]any{
		"timezoneNote": "all timestamps are in UTC (Z suffix)",
	}
	var response map[string]any

	if useRollup {
		body := map[string]any{}
		if from != "" || to != "" {
			rangeBody, err := civilRange(from, to)
			if err != nil {
				return nil, fmt.Errorf("invalid date range: %w", err)
			}
			body["range"] = rangeBody
		}
		raw, err := client.DailyRollUp(ctx, dt.EndpointName, body)
		if err != nil {
			return nil, fmt.Errorf("API error: %w", err)
		}
		raw["meta"] = meta
		response = raw
	} else if hasList {
		if pageToken != "" {
			listOpts := healthapi.ListOptions{PageToken: pageToken, PageSize: 500}
			if pageSize > 0 {
				listOpts.PageSize = pageSize
			}
			if dt.Filterable && from != "" {
				listOpts.Filter = registry.FilterFromRange(dt, from, to)
			}
			raw, err := client.ListDataPoints(ctx, dt.EndpointName, listOpts)
			if err != nil {
				return nil, fmt.Errorf("API error: %w", err)
			}
			nextToken, _ := raw["nextPageToken"].(string)
			pts, _ := raw["dataPoints"].([]any)
			meta["totalCount"] = len(pts)
			if nextToken != "" {
				meta["hasMore"] = true
				meta["nextPageToken"] = nextToken
			}
			raw["meta"] = meta
			opts := output.Options{
				Units:   units,
				Flatten: flatten,
			}
			transformed, _ := output.Transform(raw, opts).(map[string]any)
			if transformed == nil {
				transformed = raw
			}
			return transformed, nil
		}

		filter := registry.FilterFromRange(dt, from, to)

		if filter == "" && dt.ClientTimePath != "" && (from != "" || to != "") {
			fromTime, err := clientfilter.ParseBound(from)
			if err != nil {
				return nil, fmt.Errorf("invalid from: %w", err)
			}
			toTime, err := clientfilter.ParseBound(to)
			if err != nil {
				return nil, fmt.Errorf("invalid to: %w", err)
			}
			listOpts := healthapi.ListOptions{PageSize: 500}
			if pageSize > 0 {
				listOpts.PageSize = pageSize
			}
			all, err := client.ListAllDataPoints(ctx, dt.EndpointName, listOpts)
			if err != nil {
				return nil, fmt.Errorf("API error: %w", err)
			}
			pts, ok := all["dataPoints"].([]any)
			if !ok {
				return nil, fmt.Errorf("unexpected API response: missing dataPoints")
			}
			filtered, skipped := clientfilter.FilterDataPoints(pts, dt.ClientTimePath, fromTime, toTime)
			meta["unfilteredCount"] = len(pts)
			meta["filteredCount"] = len(filtered)
			meta["skippedCount"] = skipped
			response = map[string]any{"dataPoints": filtered, "meta": meta}
		} else {
			listOpts := healthapi.ListOptions{Filter: filter, PageSize: 500}
			if pageSize > 0 {
				listOpts.PageSize = pageSize
			}
			raw, err := client.ListDataPoints(ctx, dt.EndpointName, listOpts)
			if err != nil {
				return nil, fmt.Errorf("API error: %w", err)
			}
			nextToken, _ := raw["nextPageToken"].(string)
			pts, _ := raw["dataPoints"].([]any)
			meta["totalCount"] = len(pts)
			if nextToken != "" {
				meta["hasMore"] = true
				meta["nextPageToken"] = nextToken
			}
			raw["meta"] = meta
			response = raw
		}
	}

	opts := output.Options{
		Units:   units,
		Flatten: flatten,
	}
	transformed, _ := output.Transform(response, opts).(map[string]any)
	if transformed == nil {
		transformed = response
	}
	return transformed, nil
}

func main() {
	s := server.NewMCPServer(
		"ghealth-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	getCapabilitiesTool := mcp.NewTool("get_health_capabilities",
		mcp.WithDescription("Returns a list of all 31 Google Health data types and their supported operations (list, rollup, serverFilter, clientFilter). Use this before querying data to avoid unsupported operations."),
	)

	s.AddTool(getCapabilitiesTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		caps := registry.Capabilities()

		var sb strings.Builder
		sb.WriteString("Supported Google Health Data Types Capabilities:\n\n")
		for _, cap := range caps {
			fmt.Fprintf(&sb, "- **%s**: list=%v, rollup=%v, serverFilter=%v, clientFilter=%v\n",
				cap.Type, cap.List, cap.Rollup, cap.ServerFilter, cap.ClientFilter)
		}

		return mcp.NewToolResultText(sb.String()), nil
	})

	getHealthDataTool := mcp.NewTool("get_health_data",
		mcp.WithDescription("Fetches health data for a specific type. Use get_health_capabilities first to verify the type supports 'list' or 'rollup'."),
		mcp.WithString("data_type",
			mcp.Required(),
			mcp.Description("The Google Health data type (e.g., 'weight', 'sleep', 'exercise', 'active-zone-minutes')."),
		),
		mcp.WithString("from",
			mcp.Description("Start date (YYYY-MM-DD) or datetime. Format depends on the type's filter mode."),
		),
		mcp.WithString("to",
			mcp.Description("End date (YYYY-MM-DD) or datetime."),
		),
		mcp.WithBoolean("flatten",
			mcp.Description("If true, returns a simplified, flat structure (e.g., total AZM instead of nested zones). Recommended for agents."),
		),
		mcp.WithString("units",
			mcp.Description("Measurement units: 'metric' (default) or 'imperial' (adds weightLbs, distanceMiles)."),
		),
		mcp.WithBoolean("rollup",
			mcp.Description("If true, uses the dailyRollUp endpoint to return aggregated daily values instead of raw per-minute data. Recommended for multi-date queries on steps, distance, and active-zone-minutes."),
		),
		mcp.WithString("page_token",
			mcp.Description("Pagination token from a previous response's nextPageToken. When set, fetches the next page. Pass the same from/to values as the original query for filterable types."),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Number of results per page for list queries. Default 500."),
		),
	)

	s.AddTool(getHealthDataTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			args = make(map[string]any)
		}

		dataType, _ := args["data_type"].(string)
		from, _ := args["from"].(string)
		to, _ := args["to"].(string)
		flatten, _ := args["flatten"].(bool)
		units, _ := args["units"].(string)
		rollup, _ := args["rollup"].(bool)
		pageToken, _ := args["page_token"].(string)
		pageSizeFloat, _ := args["page_size"].(float64)
		pageSize := int(pageSizeFloat)
		if pageSize < 0 {
			return mcp.NewToolResultError("page_size must be a positive number."), nil
		}
		if pageSize > maxPageSize {
			return mcp.NewToolResultError(fmt.Sprintf("page_size must not exceed %d.", maxPageSize)), nil
		}

		if from != "" && !strings.Contains(from, "T") {
			if _, err := time.Parse("2006-01-02", from); err != nil {
				return mcp.NewToolResultError("from must be YYYY-MM-DD or a datetime string containing 'T'."), nil
			}
		}
		if to != "" && !strings.Contains(to, "T") {
			if _, err := time.Parse("2006-01-02", to); err != nil {
				return mcp.NewToolResultError("to must be YYYY-MM-DD or a datetime string containing 'T'."), nil
			}
		}

		dt, ok := registry.Lookup(dataType)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("Unknown data type: %s. Run get_health_capabilities to see valid types.", dataType)), nil
		}

		client, err := newHealthClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("Failed to initialize API client."), nil
		}

		result, err := fetchAndTransform(ctx, client, dt, from, to, flatten, units, rollup, pageSize, pageToken)
		if err != nil {
			log.Printf("ghealth-mcp: get_health_data fetchAndTransform error: %v", err)
			return mcp.NewToolResultError("Failed to process health data."), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			log.Printf("ghealth-mcp: get_health_data serialization error: %v", err)
			return mcp.NewToolResultError("Failed to serialize API response."), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	})

	dailySummaryTypes := []string{
		"steps", "daily-resting-heart-rate", "daily-heart-rate-variability",
		"weight", "active-zone-minutes", "distance",
		"daily-oxygen-saturation", "daily-respiratory-rate",
		"sleep", "exercise",
	}

	getDailySummaryTool := mcp.NewTool("get_daily_summary",
		mcp.WithDescription("Fetches a daily health overview for a single date, returning compact summaries for key metrics (steps, resting HR, HRV, weight, AZM, distance, SpO2, respiratory rate) plus sleep stage breakdown and exercise count/types. Sleep/exercise fields are null when no data exists for the date. Uses dailyRollUp for eligible types and client-side date filtering for daily types. Optionally specify a custom comma-separated list of types."),
		mcp.WithString("date",
			mcp.Required(),
			mcp.Description("The date to summarize (YYYY-MM-DD)."),
		),
		mcp.WithString("types",
			mcp.Description("Comma-separated list of data types to include. Defaults to all common types. Example: 'steps,weight,sleep'"),
		),
		mcp.WithString("units",
			mcp.Description("Measurement units: 'metric' (default) or 'imperial'."),
		),
	)

	s.AddTool(getDailySummaryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok {
			args = make(map[string]any)
		}
		date, _ := args["date"].(string)
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return mcp.NewToolResultError("date must be in YYYY-MM-DD format."), nil
		}
		typesParam, _ := args["types"].(string)
		units, _ := args["units"].(string)

		if len(typesParam) > maxTypesParamLen {
			return mcp.NewToolResultError(fmt.Sprintf("types parameter must not exceed %d characters.", maxTypesParamLen)), nil
		}

		client, err := newHealthClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("Failed to initialize API client."), nil
		}

		summaryCtx, cancel := context.WithTimeout(ctx, dailySummaryTimeout)
		defer cancel()

		typesToFetch := dailySummaryTypes
		if typesParam != "" {
			parts := strings.Split(typesParam, ",")
			if len(parts) > maxTypesCount {
				return mcp.NewToolResultError(fmt.Sprintf("types parameter must not contain more than %d types.", maxTypesCount)), nil
			}
			typesToFetch = make([]string, 0, len(parts))
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					typesToFetch = append(typesToFetch, trimmed)
				}
			}
		}

		data := make(map[string]any, len(typesToFetch))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, typeName := range typesToFetch {
			wg.Add(1)
			go func(typeName string) {
				defer wg.Done()
				dt, ok := registry.Lookup(typeName)
				if !ok {
					mu.Lock()
					data[typeName] = map[string]any{"error": "unknown data type"}
					mu.Unlock()
					return
				}
				useRollup := registry.HasOperation(dt, "dailyRollUp")
				result, err := fetchAndTransform(summaryCtx, client, dt, date, date, true, units, useRollup, 0, "")
				if err != nil {
					log.Printf("ghealth-mcp: get_daily_summary error for %s: %v", typeName, err)
					mu.Lock()
					data[typeName] = map[string]any{"error": "failed to fetch data"}
					mu.Unlock()
					return
				}
				mu.Lock()
				data[typeName] = result
				mu.Unlock()
			}(typeName)
		}
		wg.Wait()

		if sleepResult, ok := data["sleep"].(map[string]any); ok {
			if _, isError := sleepResult["error"]; !isError {
				data["sleep"] = extractSleepSummary(sleepResult)
			}
		}
		if exerciseResult, ok := data["exercise"].(map[string]any); ok {
			if _, isError := exerciseResult["error"]; !isError {
				data["exercise"] = extractExerciseSummary(exerciseResult)
			}
		}

		response := map[string]any{"date": date, "data": data}
		jsonBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("ghealth-mcp: get_daily_summary serialization error: %v", err)
			return mcp.NewToolResultError("Failed to serialize API response."), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	})

	log.Println("Starting ghealth-mcp server on stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
