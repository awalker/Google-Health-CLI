package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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

// newHealthClient loads the local config, creates an OAuth token source,
// and returns an authenticated healthapi.Client.
func newHealthClient() (*healthapi.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	source, err := auth.TokenSource(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("authentication error: %w", err)
	}

	httpClient := oauth2.NewClient(context.Background(), source)
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return healthapi.New(cfg.BaseURL, cfg.User, httpClient), nil
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

func main() {
	s := server.NewMCPServer(
		"ghealth-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	getCapabilitiesTool := mcp.NewTool("get_health_capabilities",
		mcp.WithDescription("Returns a list of all 31 Google Health data types and their supported operations (list, rollup, serverFilter, clientFilter). Use this before querying data to avoid unsupported operations."),
	)

	s.AddTool(getCapabilitiesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		caps := registry.Capabilities()

		result := "Supported Google Health Data Types Capabilities:\n\n"
		for _, cap := range caps {
			result += fmt.Sprintf("- **%s**: list=%v, rollup=%v, serverFilter=%v, clientFilter=%v\n",
				cap.Type, cap.List, cap.Rollup, cap.ServerFilter, cap.ClientFilter)
		}

		return mcp.NewToolResultText(result), nil
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

		dt, ok := registry.Lookup(dataType)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("Unknown data type: %s. Run get_health_capabilities to see valid types.", dataType)), nil
		}

		hasList := registry.HasOperation(dt, "list")
		hasRollup := registry.HasOperation(dt, "dailyRollUp")

		if !hasList && !hasRollup {
			return mcp.NewToolResultError(fmt.Sprintf("Data type %s does not support list or rollup.", dataType)), nil
		}

		if from != "" && from == to && !strings.Contains(from, "T") {
			t, err := time.Parse("2006-01-02", from)
			if err == nil {
				to = t.AddDate(0, 0, 1).Format("2006-01-02")
			}
		}

		if !strings.Contains(from, "T") && strings.Contains(dt.DefaultTimePath, "physical_time") {
			if from != "" {
				from += "T00:00:00Z"
			}
			if to != "" && !strings.Contains(to, "T") {
				to += "T00:00:00Z"
			}
		}

		client, err := newHealthClient()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create API client: %v", err)), nil
		}

		meta := map[string]any{
			"timezoneNote": "all timestamps are in UTC (Z suffix)",
		}
		var response map[string]any

		if hasList {
			filter := registry.FilterFromRange(dt, from, to)

			if filter == "" && dt.ClientTimePath != "" && (from != "" || to != "") {
				fromTime, err := clientfilter.ParseBound(from)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid from: %v", err)), nil
				}
				toTime, err := clientfilter.ParseBound(to)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid to: %v", err)), nil
				}
				all, err := client.ListAllDataPoints(ctx, dt.EndpointName, healthapi.ListOptions{PageSize: 500})
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
				}
				pts, ok := all["dataPoints"].([]any)
				if !ok {
					return mcp.NewToolResultError("unexpected API response: missing dataPoints"), nil
				}
				filtered, skipped := clientfilter.FilterDataPoints(pts, dt.ClientTimePath, fromTime, toTime)
				meta["unfilteredCount"] = len(pts)
				meta["filteredCount"] = len(filtered)
				meta["skippedCount"] = skipped
				response = map[string]any{"dataPoints": filtered, "meta": meta}
			} else {
				raw, err := client.ListDataPoints(ctx, dt.EndpointName, healthapi.ListOptions{Filter: filter, PageSize: 500})
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
				}
				nextToken, _ := raw["nextPageToken"].(string)
				pts, _ := raw["dataPoints"].([]any)
				meta["totalCount"] = len(pts)
				if nextToken != "" {
					meta["truncated"] = true
					meta["hint"] = "narrow the date range to get the full result set"
				}
				raw["meta"] = meta
				response = raw
			}
		} else {
			body := map[string]any{}
			if from != "" || to != "" {
				rangeBody, err := civilRange(from, to)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid date range: %v", err)), nil
				}
				body["range"] = rangeBody
			}
			raw, err := client.DailyRollUp(ctx, dt.EndpointName, body)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
			}
			raw["meta"] = meta
			response = raw
		}

		opts := output.Options{
			Units:   units,
			Flatten: flatten,
		}
		transformed := output.Transform(response, opts)

		jsonBytes, err := json.Marshal(transformed)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	})

	log.Println("Starting ghealth-mcp server on stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
