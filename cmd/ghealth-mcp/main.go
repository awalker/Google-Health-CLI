package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rudrankriyam/Google-Health-CLI/internal/registry"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"ghealth-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Tool 1: Get Health Capabilities
	// Returns a clean, low-token matrix of what each data type supports
	getCapabilitiesTool := mcp.NewTool("get_health_capabilities",
		mcp.WithDescription("Returns a list of all 31 Google Health data types and their supported operations (list, rollup, serverFilter, clientFilter). Use this before querying data to avoid unsupported operations."),
	)

	s.AddTool(getCapabilitiesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		caps := registry.Capabilities()
		
		// Format as a clean, readable text response for the LLM, or JSON
		// Using JSON here so the LLM can parse it easily if needed, but MCP text is often safer
		result := "Supported Google Health Data Types Capabilities:\n\n"
		for _, cap := range caps {
			result += fmt.Sprintf("- **%s**: list=%v, rollup=%v, serverFilter=%v, clientFilter=%v\n",
				cap.Type, cap.List, cap.Rollup, cap.ServerFilter, cap.ClientFilter)
		}
		
		return mcp.NewToolResultText(result), nil
	})

	// Tool 2: Get Health Data (Stub for demonstration of parameter handling)
	// In the next iteration, this will wire up to internal/healthapi and internal/output
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

		// Validate data type exists using our existing registry
		dt, ok := registry.Lookup(dataType)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("Unknown data type: %s. Run get_health_capabilities to see valid types.", dataType)), nil
		}

		// TODO: Wire up internal/healthapi.Client.ListDataPoints or ListAllDataPoints here
		// TODO: Pass results through internal/output.Transform with Flatten and Units options

		response := fmt.Sprintf("[STUB] Successfully validated request for type: '%s' (Filterable: %v, ClientTimePath: %v). "+
			"Date range: %s to %s. Flatten: %v, Units: %s. \n\n"+
			"Next step: Wire this tool to internal/healthapi and internal/output to fetch and transform the actual data.",
			dt.EndpointName, dt.Filterable, dt.ClientTimePath != "", from, to, flatten, units)

		return mcp.NewToolResultText(response), nil
	})

	log.Println("Starting ghealth-mcp server on stdio...")
	// Start the stdio server (blocks)
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
