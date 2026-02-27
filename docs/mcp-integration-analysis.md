# MCP Integration Analysis: All the Ways to Serve Isthmus

## Current State

Isthmus currently serves via a single transport: **MCP over stdio** (JSON-RPC on stdin/stdout). This is the standard for local MCP servers consumed by AI-native clients like Claude Desktop, Claude Code, or Cursor.

```go
// cmd/isthmus/main.go — current setup
stdioServer := mcpserver.NewStdioServer(mcpServer)
stdioServer.Listen(ctx, os.Stdin, os.Stdout)
```

Below is a comprehensive analysis of **every way** Isthmus can be served and consumed — from what you already have to what you could add.

---

## 1. Stdio Transport (Current)

**How it works:** The host application (Claude Desktop, Cursor, etc.) spawns `isthmus` as a child process and communicates via JSON-RPC over stdin/stdout.

**Who can use it:**
- Claude Desktop
- Claude Code (CLI)
- Cursor, Windsurf, Zed, and other IDE integrations
- Any MCP-aware client that manages subprocess lifecycle

**Configuration example (Claude Desktop):**
```json
{
  "mcpServers": {
    "isthmus": {
      "command": "isthmus",
      "env": {
        "DATABASE_URL": "postgres://user:pass@localhost:5432/mydb"
      }
    }
  }
}
```

**Pros:** Zero network exposure, credentials stay local, simple.
**Cons:** Client and server must run on the same machine. Can't be shared across a network.

---

## 2. HTTP + SSE Transport (Available in mcp-go SDK)

The `mcp-go` v0.44.0 SDK already includes `server.NewSSEServer()`, which serves MCP over HTTP using Server-Sent Events for the server→client channel and POST requests for client→server messages.

**What you would change:**

```go
// Instead of stdioServer, add an HTTP/SSE option:
import mcpserver "github.com/mark3labs/mcp-go/server"

sseServer := mcpserver.NewSSEServer(mcpServer,
    mcpserver.WithBaseURL("http://localhost:8080"),
)
log.Fatal(sseServer.Start(":8080"))
```

**Who can use it:**
- Remote MCP clients over the network
- Web applications running MCP client SDKs
- Multiple AI applications sharing a single Isthmus instance

**Pros:** Network-accessible, shareable across machines/users, can sit behind a reverse proxy with auth.
**Cons:** Requires network security considerations (TLS, authentication).

---

## 3. Streamable HTTP Transport (Available in mcp-go SDK)

The newest MCP transport, also supported by `mcp-go`. Uses a single HTTP endpoint with bidirectional streaming — simpler than SSE and designed for stateless/serverless environments.

```go
httpServer := mcpserver.NewStreamableHTTPServer(mcpServer)
log.Fatal(httpServer.Start(":8080"))
```

**Who can use it:** Same as SSE, but better suited for cloud deployments, serverless functions, and load-balanced environments.

**Pros:** Simpler than SSE, stateless-friendly, works behind CDNs/load balancers.
**Cons:** Newer spec — some clients may not support it yet.

---

## 4. Programmatic Use from Application Code (MCP Client SDKs)

**This is the core of your question.** Yes — an application using any AI API (Anthropic, OpenAI, etc.) can absolutely communicate with Isthmus programmatically by using an MCP client SDK.

### How It Works

The pattern is:

```
Your App  →  AI API (e.g., Anthropic)  →  returns tool_use decisions
   ↓
Your App  →  MCP Client SDK  →  connects to Isthmus  →  executes tool
   ↓
Your App  →  sends tool results back to AI API  →  gets final response
```

Your application acts as the **orchestrator** — it talks to both the AI API and the MCP server, bridging the two.

### Example: TypeScript Application Using Anthropic API + MCP Client

```typescript
import Anthropic from "@anthropic-ai/sdk";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
// or: import { SSEClientTransport } from "@modelcontextprotocol/sdk/client/sse.js";

// 1. Connect to Isthmus via MCP client
const transport = new StdioClientTransport({
  command: "isthmus",
  env: { DATABASE_URL: "postgres://..." },
});
const mcpClient = new Client({ name: "my-app", version: "1.0" });
await mcpClient.connect(transport);

// 2. Discover available tools
const { tools } = await mcpClient.listTools();

// 3. Convert MCP tools to Anthropic tool format
const anthropicTools = tools.map((tool) => ({
  name: tool.name,
  description: tool.description,
  input_schema: tool.inputSchema,
}));

// 4. Send user query to Anthropic with Isthmus tools
const anthropic = new Anthropic();
const response = await anthropic.messages.create({
  model: "claude-sonnet-4-20250514",
  max_tokens: 1024,
  tools: anthropicTools,
  messages: [{ role: "user", content: "What tables are in my database?" }],
});

// 5. If Claude wants to call a tool, route it to Isthmus
for (const block of response.content) {
  if (block.type === "tool_use") {
    const result = await mcpClient.callTool({
      name: block.name,
      arguments: block.input,
    });
    // 6. Feed the result back to Claude for the final answer
  }
}
```

### Example: Python Application Using Anthropic API + MCP Client

```python
import anthropic
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

# 1. Connect to Isthmus
server_params = StdioServerParameters(
    command="isthmus",
    env={"DATABASE_URL": "postgres://..."},
)

async with stdio_client(server_params) as (read, write):
    async with ClientSession(read, write) as session:
        await session.initialize()

        # 2. List available tools
        tools = await session.list_tools()

        # 3. Use with Anthropic API
        client = anthropic.Anthropic()
        response = client.messages.create(
            model="claude-sonnet-4-20250514",
            tools=[{
                "name": t.name,
                "description": t.description,
                "input_schema": t.inputSchema,
            } for t in tools.tools],
            messages=[{"role": "user", "content": "Show me all tables"}],
        )

        # 4. Route tool calls to Isthmus
        for block in response.content:
            if block.type == "tool_use":
                result = await session.call_tool(block.name, block.input)
                # Feed result back to Claude...
```

### Example: Go Application (Direct Import — No MCP Protocol Needed)

Because Isthmus uses hexagonal architecture with clean port interfaces, a Go application can **bypass MCP entirely** and import the core packages directly:

```go
package main

import (
    "github.com/guillermoBallester/isthmus/internal/adapter/postgres"
    "github.com/guillermoBallester/isthmus/internal/core/domain"
    "github.com/guillermoBallester/isthmus/internal/core/service"
)

func main() {
    pool, _ := postgres.NewPool(ctx, "postgres://...")

    explorer := postgres.NewExplorer(pool, nil)
    executor := postgres.NewExecutor(pool, true, 100, 10*time.Second)
    validator := domain.NewPgQueryValidator()
    querySvc := service.NewQueryService(validator, executor, audit.NoopAuditor{}, logger)

    // Use directly — no MCP protocol overhead
    tables, _ := explorer.ListTables(ctx)
    result, _ := querySvc.Execute(ctx, "SELECT * FROM users LIMIT 10")
}
```

**Is this useful?** Absolutely. Use cases include:
- Building a custom chatbot that queries your database
- Internal dashboards with AI-powered data exploration
- Slack/Discord bots that answer questions about production data
- CI/CD pipelines that validate schema changes using AI analysis
- Backend services that need schema-aware SQL generation

---

## 5. REST API Wrapper (Custom, Non-MCP)

You could expose the same tools as a traditional REST API, entirely independent of MCP:

```
POST /api/v1/tools/list_tables
POST /api/v1/tools/describe_table    { "table_name": "users" }
POST /api/v1/tools/query             { "sql": "SELECT ..." }
```

This wouldn't use the MCP protocol at all — just standard HTTP/JSON. Any application (AI-powered or not) could call it.

**When this makes sense:** If you want Isthmus to serve non-AI applications, internal tools, or clients that don't support MCP at all.

---

## 6. Docker / Sidecar Deployment

You already have a `Dockerfile`. Combined with HTTP transports (SSE or Streamable HTTP), Isthmus can be deployed as:

- A **Docker sidecar** in Kubernetes alongside application pods
- A **standalone service** on your internal network
- A **cloud-hosted instance** (Cloud Run, ECS, Fly.io, etc.)

```yaml
# docker-compose.yml addition
services:
  isthmus:
    build: .
    environment:
      DATABASE_URL: postgres://user:pass@db:5432/mydb
    ports:
      - "8080:8080"  # if using HTTP/SSE transport
```

---

## 7. Claude Agent SDK (claude-agent-sdk / @anthropic-ai/claude-code)

Anthropic's Agent SDK can spawn and manage MCP servers directly. An agent built with the SDK could use Isthmus as one of its tool providers:

```typescript
import { Agent } from "@anthropic-ai/claude-code";

const agent = new Agent({
  mcpServers: {
    isthmus: {
      command: "isthmus",
      env: { DATABASE_URL: "postgres://..." },
    },
  },
});

// The agent can now use list_tables, query, etc. as tools
const result = await agent.run("Analyze the users table schema");
```

---

## Summary: All Serving Options at a Glance

| # | Transport / Method | Network | Multi-client | Code Changes | Best For |
|---|-------------------|---------|-------------|-------------|----------|
| 1 | **Stdio** (current) | Local only | No | None | Claude Desktop, IDEs |
| 2 | **HTTP + SSE** | Yes | Yes | ~10 lines | Shared/remote MCP clients |
| 3 | **Streamable HTTP** | Yes | Yes | ~5 lines | Cloud/serverless deployments |
| 4 | **MCP Client SDK** (programmatic) | Either | N/A | Client-side only | Custom AI apps (any language) |
| 5 | **Direct Go import** | In-process | N/A | None (library use) | Go applications |
| 6 | **REST API wrapper** | Yes | Yes | New adapter needed | Non-MCP / non-AI consumers |
| 7 | **Docker sidecar** | Container network | Yes | Combine with #2 or #3 | Kubernetes, cloud deployments |
| 8 | **Agent SDK** | Local | No | Client-side only | Building AI agents |

---

## Recommendation

The highest-impact addition would be **adding HTTP+SSE or Streamable HTTP transport** (options 2/3). This is a small change (~10 lines in `main.go` + a new CLI flag like `--transport http --port 8080`) and it unlocks:

- Programmatic access from any language via MCP client SDKs over the network
- Shared instances for teams (one Isthmus, many consumers)
- Cloud deployment scenarios
- All the use cases in option 4, but without needing to run on the same machine

The `mcp-go` SDK you already depend on supports both transports — no new dependencies required.
