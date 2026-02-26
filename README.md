```
  _     _   _
 (_)___| |_| |__  _ __ ___  _   _ ___
 | / __| __| '_ \| '_ ` _ \| | | / __|
 | \__ \ |_| | | | | | | | | |_| \__ \
 |_|___/\__|_| |_|_| |_| |_|\__,_|___/

 Your database, understood by AI.
```

[![CI](https://github.com/guillermoBallester/isthmus/actions/workflows/ci.yml/badge.svg)](https://github.com/guillermoBallester/isthmus/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![MCP](https://img.shields.io/badge/MCP-2025--03--26-blueviolet)](https://modelcontextprotocol.io)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

---

**Give Claude (or any LLM) safe, read-only access to your PostgreSQL database.**

One binary. Runs locally. Your credentials never leave your machine.

## How It Works

```
┌──────────────────────────────────────────────────────┐
│  Your Machine                                         │
│                                                       │
│  Claude Desktop / Cursor / VS Code                    │
│       │                                               │
│       │ stdio (MCP protocol)                          │
│       ▼                                               │
│  ┌─────────────────────────┐                          │
│  │  isthmus                │                          │
│  │                         │                          │
│  │  • Schema explorer      │     ┌──────────────┐    │
│  │  • SQL validator        │────▶│ PostgreSQL    │    │
│  │  • Query executor       │     │ (any host)    │    │
│  │  • Policy engine        │     └──────────────┘    │
│  │  • Business context     │                          │
│  └─────────────────────────┘                          │
│                                                       │
└──────────────────────────────────────────────────────┘
```

Isthmus connects to your database and serves MCP tools over stdio. The LLM can explore your schema, understand table relationships, and run read-only queries — all through the standard MCP protocol.

## Quick Start

### 1. Build

```bash
go install github.com/guillermoBallester/isthmus/cmd/isthmus@latest
# or
make build    # -> bin/isthmus
```

### 2. Configure Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

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

### 3. Ask questions

Open Claude Desktop and ask:

> "What tables are in my database?"
> "Show me the schema for the orders table"
> "What are the top 10 customers by total order value?"

## MCP Tools

| Tool | Description |
|------|-------------|
| `list_schemas` | List all available database schemas |
| `list_tables` | List tables with schema, type, row count, and comments |
| `describe_table` | Column details, primary keys, foreign keys, indexes |
| `query` | Execute read-only SQL, returns JSON array of objects |
| `explain_query` | Show PostgreSQL execution plan (supports ANALYZE) |

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `READ_ONLY` | `true` | Wrap queries in read-only transactions |
| `MAX_ROWS` | `100` | Server-side row limit on all queries |
| `QUERY_TIMEOUT` | `10s` | Per-query execution timeout |
| `SCHEMAS` | *(all)* | Comma-separated schema allowlist (e.g. `public,app`) |
| `POLICY_FILE` | *(none)* | Path to policy YAML for business context and access control |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

## Policy Engine

Enrich your schema with business context so the LLM understands what your data means:

```yaml
# policy.yaml
context:
  tables:
    public.customers:
      description: "Customer accounts. One row per paying customer."
      columns:
        mrr:
          description: "Monthly Recurring Revenue in cents"
        plan_tier:
          description: "Subscription tier: free, starter, pro, enterprise"
    public.orders:
      description: "Purchase orders. Links to customers via customer_id."
```

```json
{
  "mcpServers": {
    "isthmus": {
      "command": "isthmus",
      "env": {
        "DATABASE_URL": "postgres://...",
        "POLICY_FILE": "./policy.yaml"
      }
    }
  }
}
```

When the LLM calls `describe_table`, it gets column types AND your business descriptions — leading to better SQL generation and more accurate answers.

## Safety & Security

- **Local only** — runs on your machine, credentials never leave
- **Read-only transactions** — queries run inside `SET TRANSACTION READ ONLY`
- **Row limits** — server-side `LIMIT` injection, independent of LLM-generated SQL
- **Query timeout** — `context.WithTimeout` on every execution
- **SQL validation** — whitelist: `SELECT` and `EXPLAIN` only, enforced at AST level via `pg_query` parser
- **Schema filtering** — restrict visibility to specific schemas via allowlist
- **Policy engine** — table/column filtering and business context via YAML config
- **Open source** — every line of code is auditable

## Architecture

```
isthmus/
├── cmd/isthmus/                     # Binary entrypoint (stdio MCP server)
│
├── internal/
│   ├── core/                        # Business logic (no external dependencies)
│   │   ├── domain/                  #   SQL validation (pg_query AST whitelist)
│   │   ├── port/                    #   Interfaces: SchemaExplorer, QueryExecutor
│   │   └── service/                 #   Application services (QueryService, ExplorerService)
│   │
│   ├── adapter/                     # Infrastructure implementations
│   │   ├── postgres/                #   PostgreSQL: executor, explorer, connection pool
│   │   └── mcp/                     #   MCP server: tool registration, handlers, hooks
│   │
│   ├── policy/                      # Policy engine: YAML loading, context enrichment
│   └── config/                      # Environment variable loading
│
├── docs/design/                     # Architecture decision records
├── Dockerfile                       # Container image
└── docker-compose.yml               # Local Postgres for tests
```

**Hexagonal architecture** (ports & adapters). Core business logic defines interfaces (`port/`) and pure domain rules (`domain/`). Infrastructure details (PostgreSQL, MCP protocol) live in `adapter/`.

## Development

### Prerequisites

- Go 1.24+
- Docker (for integration tests)

### Commands

```bash
make build       # Build binary -> bin/isthmus
make test        # Run all tests (requires Docker for testcontainers)
make test-short  # Unit tests only
make lint        # golangci-lint
make vet         # go vet
make fmt         # gofmt
make tidy        # go mod tidy
make clean       # Remove bin/
```

### Testing

Unit tests run without Docker. Integration tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up real PostgreSQL instances.

```bash
go test -short -race -count=1 ./...   # Unit tests only
go test -race -count=1 ./...          # All tests (needs Docker)
```

## Coming Soon

- **Schema profiler** — auto-discover column statistics, enum values, relationships
- **Column masking** — hide sensitive columns (emails, SSNs) in query results
- **Cloud dashboard** — optional team features: schema browser, change tracking, shared annotations
- **MySQL adapter** — second database backend

## License

[Apache 2.0](LICENSE)
