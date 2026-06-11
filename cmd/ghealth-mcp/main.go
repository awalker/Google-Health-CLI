package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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

		if !registry.HasOperation(dt, "list") {
			return mcp.NewToolResultError(fmt.Sprintf("Data type %s does not support the list operation.", dataType)), nil
		}

		client, err := newHealthClient()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create API client: %v", err)), nil
		}

		var response map[string]any
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
			response = map[string]any{"dataPoints": filtered, "meta": map[string]any{
				"unfilteredCount": len(pts),
				"filteredCount":   len(filtered),
				"skippedCount":    skipped,
			}}
		} else {
			response, err = client.ListDataPoints(ctx, dt.EndpointName, healthapi.ListOptions{Filter: filter})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
			}
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
