# Session Handoff: MCP Server Scaffold

## Branch Status
- **Branch**: `feat/mcp-server-scaffold`
- **Base**: `dev`
- **Status**: Scaffold complete, dependencies added, ready for OpenCode testing and final API wiring.

## What Was Built

1. **New Entry Point**: `cmd/ghealth-mcp/main.go`
   - Uses `github.com/mark3labs/mcp-go` to create a stdio-based MCP server.
   - **Tool 1: `get_health_capabilities`**: Fully wired. Calls `registry.Capabilities()` and returns a clean, LLM-friendly summary of all 31 data types and their supported operations (`list`, `rollup`, `serverFilter`, `clientFilter`).
   - **Tool 2: `get_health_data`**: Parameter-validated stub. Accepts `data_type`, `from`, `to`, `flatten`, and `units`. Validates the type against `registry.Lookup`. *Note: Does not yet fetch actual data; returns a confirmation stub.*

2. **Build System**: `Makefile` updated to build both binaries:
   ```make
   build:
       go build -o ghealth .
       go build -o ghealth-mcp ./cmd/ghealth-mcp
   ```

3. **Dependencies**: `go.mod` updated with `github.com/mark3labs/mcp-go` and required indirect dependencies (jsonschema, uuid, etc.).

## How to Test Locally

1. **Build the binaries**:
   ```bash
   cd /home/walke/Projects/Google-Health-CLI
   make build
   ```

2. **Test the MCP server manually** (optional, but good for debugging stdio):
   ```bash
   # The server runs on stdio, so it will wait for JSON-RPC input. 
   # You can send a simple initialization or tool call to verify it doesn't crash on startup.
   ./ghealth-mcp
   ```
   *(Note: For real testing, it's best to configure it in an MCP host like Hermes or OpenCode).*

3. **Configure in an MCP Host** (e.g., Hermes `~/.hermes/config.yaml` or OpenCode equivalent):
   ```yaml
   mcp_servers:
     ghealth:
       command: "/home/walke/Projects/Google-Health-CLI/ghealth-mcp"
   ```

## What's Missing (Next Steps for OpenCode)

The `get_health_data` tool is currently a stub. To make it fully functional, the OpenCode session needs to:

1. **Import the existing internal packages** in `cmd/ghealth-mcp/main.go`:
   ```go
   import (
       "github.com/rudrankriyam/Google-Health-CLI/internal/healthapi"
       "github.com/rudrankriyam/Google-Health-CLI/internal/output"
       "github.com/rudrankriyam/Google-Health-CLI/internal/config"
   )
   ```

2. **Initialize the Health API Client**: Inside the `get_health_data` tool handler, load the config and create the client:
   ```go
   cfg, _ := config.Load()
   client := healthapi.New(cfg.BaseURL, cfg.User, nil) // nil uses default HTTP client
   ```

3. **Fetch the Data**: 
   - If `dataType.ClientTimePath != ""` and `from`/`to` are provided, use `client.ListAllDataPoints(...)`.
   - Otherwise, use `client.ListDataPoints(...)` or `client.DailyRollUp(...)` depending on the operation.

4. **Transform the Output**: Pass the raw `map[string]any` response through the existing transformation logic:
   ```go
   opts := output.Options{
       Units:   units,   // "metric" or "imperial"
       Flatten: flatten, // true or false
   }
   transformed := output.Transform(rawResponse, opts)
   ```

5. **Return the Result**: Format `transformed` as JSON and return it via `mcp.NewToolResultText(jsonString)` or `mcp.NewToolResultStructured`.

## Key Reminders
- **Do not duplicate logic**: All filtering, pagination, unit conversion, and flattening logic already exists in `internal/clientfilter`, `internal/healthapi`, and `internal/output`. Reuse it.
- **Error Handling**: Ensure API errors (like invalid date formats or unsupported types) are caught and returned as clean `mcp.NewToolResultError` messages so the LLM understands what went wrong.
- **Go Module**: If you add new imports, remember to run `go mod tidy`.

## Git Commands for OpenCode
```bash
cd /home/walke/Projects/Google-Health-CLI
git checkout feat/mcp-server-scaffold
# Make your changes...
git add -A
git commit -m "feat: wire get_health_data to healthapi and output.Transform"
```
