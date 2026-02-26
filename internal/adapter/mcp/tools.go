package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/guillermoBallester/isthmus/internal/core/service"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server metadata
const serverName = "isthmus"

// Tool descriptions
const (
	descListSchemas = "List all available database schemas. " +
		"Call this first to discover what schemas exist before listing tables or describing them."

	descListTables = "List all tables and views with schema, type, estimated row count, total size, column count, " +
		"and whether indexes exist. Use this to understand the database landscape — " +
		"table sizes tell you which tables are large (avoid SELECT * on them) and " +
		"row estimates help you plan queries with appropriate LIMIT clauses."

	descDescribeTable = "Describe a table's full structure including: columns with types, nullability, defaults, and comments; " +
		"column-level statistics from pg_stats (cardinality classification, null rates, enum-like values with frequencies, " +
		"value ranges for dates/numbers); primary keys; foreign keys with referenced tables; indexes; " +
		"check constraints; row estimate; table size; and statistics freshness. " +
		"Use this to understand a table before writing queries. " +
		"Pay attention to: foreign keys for JOIN paths; cardinality to know what to GROUP BY vs filter; " +
		"enum-like columns show the allowed values; value ranges show date spans and numeric scales; " +
		"null rates help you handle NULLs correctly in filters and JOINs."

	descDescribeTableParam = "Name of the table to describe"

	descProfileTable = "Deep-profile a single table: sample rows, disk size breakdown (table/indexes/TOAST), " +
		"index usage statistics (which indexes are actually used vs unused), " +
		"inferred foreign key relationships (detected from naming patterns like *_id), " +
		"and statistics freshness warnings. " +
		"Use this when you need deeper analysis than describe_table provides — " +
		"for example, to see actual data patterns in sample rows, find unused indexes, " +
		"or discover implicit relationships in databases without explicit foreign keys."

	descProfileTableParam = "Name of the table to profile"

	descQuery = "Execute a read-only SQL query against the database and return results as a JSON array of objects. " +
		"A server-side row limit and query timeout are enforced. " +
		"Always use specific column names instead of SELECT *. " +
		"Use JOINs based on foreign keys discovered via describe_table. " +
		"Check column cardinality from describe_table to write efficient WHERE and GROUP BY clauses."

	descQueryParam = "SQL query to execute (SELECT statements only)"

	descExplainQuery = "Show the PostgreSQL execution plan for a SQL query. " +
		"Returns the query planner's strategy including scan types, join methods, and cost estimates. " +
		"Use this to understand query performance before or after running a query. " +
		"Supports ANALYZE to include actual execution statistics (the query WILL be executed)."

	descExplainQuerySQL = "The SELECT query to explain (without the EXPLAIN keyword)"
)

func RegisterTools(s *server.MCPServer, explorer *service.ExplorerService, profiler *service.ProfilerService, query *service.QueryService) {
	s.AddTool(
		mcp.NewTool("list_schemas",
			mcp.WithDescription(descListSchemas),
		),
		listSchemasHandler(explorer),
	)

	s.AddTool(
		mcp.NewTool("list_tables",
			mcp.WithDescription(descListTables),
		),
		listTablesHandler(explorer),
	)

	s.AddTool(
		mcp.NewTool("describe_table",
			mcp.WithDescription(descDescribeTable),
			mcp.WithString("table_name",
				mcp.Required(),
				mcp.Description(descDescribeTableParam),
			),
			mcp.WithString("schema",
				mcp.Description("Schema name (optional, resolves automatically if omitted)"),
			),
		),
		describeTableHandler(explorer),
	)

	if profiler != nil {
		s.AddTool(
			mcp.NewTool("profile_table",
				mcp.WithDescription(descProfileTable),
				mcp.WithString("table_name",
					mcp.Required(),
					mcp.Description(descProfileTableParam),
				),
				mcp.WithString("schema",
					mcp.Description("Schema name (optional, resolves automatically if omitted)"),
				),
			),
			profileTableHandler(profiler),
		)
	}

	s.AddTool(
		mcp.NewTool("query",
			mcp.WithDescription(descQuery),
			mcp.WithString("sql",
				mcp.Required(),
				mcp.Description(descQueryParam),
			),
		),
		queryHandler(query),
	)

	s.AddTool(
		mcp.NewTool("explain_query",
			mcp.WithDescription(descExplainQuery),
			mcp.WithString("sql",
				mcp.Required(),
				mcp.Description(descExplainQuerySQL),
			),
			mcp.WithBoolean("analyze",
				mcp.Description("Include actual execution statistics (executes the query). Defaults to false."),
			),
		),
		explainQueryHandler(query),
	)
}

func listSchemasHandler(explorer *service.ExplorerService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		schemas, err := explorer.ListSchemas(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list schemas: %v", err)), nil
		}

		data, err := json.Marshal(schemas)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func listTablesHandler(explorer *service.ExplorerService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tables, err := explorer.ListTables(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list tables: %v", err)), nil
		}

		data, err := json.Marshal(tables)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func describeTableHandler(explorer *service.ExplorerService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tableName, ok := request.GetArguments()["table_name"].(string)
		if !ok || tableName == "" {
			return mcp.NewToolResultError("table_name is required"), nil
		}

		schema, _ := request.GetArguments()["schema"].(string)

		detail, err := explorer.DescribeTable(ctx, schema, tableName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to describe table: %v", err)), nil
		}

		data, err := json.Marshal(detail)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func profileTableHandler(profiler *service.ProfilerService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tableName, ok := request.GetArguments()["table_name"].(string)
		if !ok || tableName == "" {
			return mcp.NewToolResultError("table_name is required"), nil
		}

		schema, _ := request.GetArguments()["schema"].(string)

		profile, err := profiler.ProfileTable(ctx, schema, tableName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to profile table: %v", err)), nil
		}

		data, err := json.Marshal(profile)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func explainQueryHandler(query *service.QueryService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sql, ok := request.GetArguments()["sql"].(string)
		if !ok || sql == "" {
			return mcp.NewToolResultError("sql is required"), nil
		}

		analyze, _ := request.GetArguments()["analyze"].(bool)

		prefix := "EXPLAIN "
		if analyze {
			prefix = "EXPLAIN ANALYZE "
		}

		ctx = service.WithToolName(ctx, "explain_query")
		results, err := query.Execute(ctx, prefix+sql)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("explain failed: %v", err)), nil
		}

		data, err := json.Marshal(results)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func queryHandler(query *service.QueryService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sql, ok := request.GetArguments()["sql"].(string)
		if !ok || sql == "" {
			return mcp.NewToolResultError("sql is required"), nil
		}

		ctx = service.WithToolName(ctx, "query")
		results, err := query.Execute(ctx, sql)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
		}

		data, err := json.Marshal(results)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}
