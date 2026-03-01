<p align="center">
<strong>Isthmus</strong>
</p>

<p align="center">
  <strong>The MCP server for your database</strong>
</p>

<p align="center">
  <a href="https://github.com/guillermoBallester/isthmus/actions/workflows/ci.yml"><img src="https://github.com/guillermoBallester/isthmus/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://goreportcard.com/report/github.com/guillermoBallester/isthmus"><img src="https://goreportcard.com/badge/github.com/guillermoBallester/isthmus" alt="Go Report Card" /></a>
  <a href="https://github.com/guillermoBallester/isthmus/releases/latest"><img src="https://img.shields.io/github/v/release/guillermoBallester/isthmus?label=release" alt="Latest Release" /></a>
  <a href="https://github.com/guillermoBallester/isthmus/stargazers"><img src="https://img.shields.io/github/stars/guillermoBallester/isthmus" alt="GitHub Stars" /></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go" /></a>
</p>

<p align="center">
  <a href="https://isthmus.dev/docs">Docs</a> &middot;
  <a href="https://isthmus.dev/docs/quickstart">Quickstart</a> &middot;
  <a href="https://isthmus.dev/docs/installation">Install</a> &middot;
  <a href="https://github.com/guillermoBallester/isthmus/issues">Issues</a>
</p>

---

Isthmus is a local [MCP](https://modelcontextprotocol.io) server that gives AI models safe, read-only access to your PostgreSQL database. One binary, runs on your machine, credentials never leave.

<!-- TODO: Replace with a terminal recording (e.g. VHS, asciinema)
<p align="center">
  <img src="docs/assets/demo.gif" alt="Isthmus demo" width="700" />
</p>
-->

## Quick start

```bash
# 1. Install
curl -fsSL https://isthmus.dev/install.sh | sh

# 2. Add to your MCP client config (Claude Desktop example)
```

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

```
# 3. Ask your AI: "What tables are in my database?"
```

See the [quickstart guide](https://isthmus.dev/docs/quickstart) for step-by-step setup with Claude Desktop, Cursor, Windsurf, and more.

## Features

- **Schema discovery** — explore schemas, tables, columns, foreign keys, and indexes ([docs](https://isthmus.dev/docs/tools/overview))
- **Read-only queries** — execute SQL with server-side row limits and query timeouts ([docs](https://isthmus.dev/docs/tools/query))
- **Column masking** — protect PII with per-column redact, hash, partial, or null masks — enforced server-side ([docs](https://isthmus.dev/features/docs/column-masking))
- **Table profiler** — column statistics, cardinality, sample rows, index usage ([docs](https://isthmus.dev/docs/tools/profile-table))
- **Policy engine** — enrich your schema with business context so the AI writes better SQL ([docs](https://isthmus.dev/docs/features/policy-engine))
- **SQL validation** — AST-level whitelist via `pg_query` parser — only `SELECT` and `EXPLAIN` allowed ([docs](https://isthmus.dev/docs/configuration))
- **HTTP transport** — serve MCP over HTTP for web-based clients, ChatGPT Desktop, and remote access ([docs](https://isthmus.dev/docs/features/http-transport))
- **OpenTelemetry** — distributed tracing and metrics for query performance and error monitoring ([docs](https://isthmus.dev/features/docs/opentelemetry))
- **Works with any MCP client** — Claude Desktop, Cursor, Windsurf, Gemini CLI, VS Code, ChatGPT Desktop ([client setup](https://isthmus.dev/docs/clients/claude-desktop))

## How it works

```mermaid
flowchart TB
    %% ── Clients ──────────────────────────────────────
    Claude["🖥️ Claude Desktop"]
    Cursor["🖥️ Cursor / VS Code"]
    ChatGPT["🌐 ChatGPT / Web"]

    Claude & Cursor -->|stdio| STDIO
    ChatGPT -->|HTTP| HTTP

    %% ── Transport ────────────────────────────────────
    subgraph Transport["⚡ Transport Layer"]
        STDIO["stdio transport"]
        HTTP["HTTP transport\n+ Bearer auth"]
    end

    STDIO & HTTP --> Router

    %% ── MCP Router ──────────────────────────────────
    subgraph MCP["🔧 MCP Tools"]
        Router{{"tool router"}}
        Router --> Discover["discover\nschemas + tables"]
        Router --> Describe["describe_table\ncolumns, keys, stats"]
        Router --> Query["query\nread-only SQL"]
    end

    %% ── Schema exploration path ─────────────────────
    Discover & Describe --> Explorer

    subgraph ExplorerLayer["🔍 Schema Explorer"]
        Explorer["PostgreSQL catalog\nintrospection"]
        Explorer --> PolicyE["Policy Engine\n+ business context"]
    end

    %% ── Query execution path ────────────────────────
    Query --> Pipeline

    subgraph SecurityPipeline["🛡️ Security Pipeline"]
        direction TB
        Pipeline["SQL Validation\nAST whitelist · pg_query parser\nSELECT & EXPLAIN only"] --> TxMode
        TxMode["Read-Only Transaction\nAccessMode: ReadOnly"] --> RowLimit
        RowLimit["Row Limit\nserver-side LIMIT wrapper"] --> Timeout
        Timeout["Query Timeout\nGo context + SET LOCAL\nstatement_timeout"]
    end

    %% ── Database ────────────────────────────────────
    SecurityPipeline --> PG[("🐘 PostgreSQL")]
    ExplorerLayer --> PG

    %% ── Post-processing ─────────────────────────────
    PG --> PostProcess

    subgraph PostProcess["🔒 Post-Processing"]
        direction TB
        Masking["PII Masking\nredact · hash · partial · null\nserver-side, per-column"]
        Masking --> ErrorSan["Error Sanitization\nno internal details leaked"]
    end

    %% ── Observability ───────────────────────────────
    PostProcess -.-> Audit["📝 Audit Log\nNDJSON append-only"]
    PostProcess -.-> OTel["📊 OpenTelemetry\ntraces + metrics"]

    %% ── Response ────────────────────────────────────
    PostProcess --> Response["✅ Safe response\nmasked · limited · validated"]
    Response --> Claude & Cursor & ChatGPT

    %% ── Styles ──────────────────────────────────────
    classDef client fill:#e8f4f8,stroke:#2196F3,color:#1565C0
    classDef transport fill:#fff3e0,stroke:#FF9800,color:#E65100
    classDef tools fill:#e8eaf6,stroke:#3F51B5,color:#283593
    classDef security fill:#fce4ec,stroke:#E53935,color:#b71c1c
    classDef explorer fill:#e8f5e9,stroke:#4CAF50,color:#1B5E20
    classDef postproc fill:#f3e5f5,stroke:#9C27B0,color:#4A148C
    classDef db fill:#fff8e1,stroke:#FFC107,color:#F57F17
    classDef obs fill:#eceff1,stroke:#607D8B,color:#37474F
    classDef response fill:#e0f2f1,stroke:#009688,color:#004D40

    class Claude,Cursor,ChatGPT client
    class STDIO,HTTP transport
    class Router,Discover,Describe,Query tools
    class Pipeline,TxMode,RowLimit,Timeout security
    class Explorer,PolicyE explorer
    class Masking,ErrorSan postproc
    class PG db
    class Audit,OTel obs
    class Response response
```

Isthmus sits between your AI client and your database. Every request flows through a **security pipeline** — SQL is validated at the AST level using PostgreSQL's own parser, queries run in read-only transactions with server-side row limits and timeouts, and PII columns are masked before results reach the AI. The **policy engine** enriches schema metadata with business context so the AI writes better SQL. All activity is recorded in an append-only audit log with optional OpenTelemetry tracing.

## MCP tools

| Tool | What it does |
|---|---|
| `list_schemas` | Discover available database schemas |
| `list_tables` | Tables with row counts, sizes, and descriptions |
| `describe_table` | Columns, types, keys, indexes, and statistics |
| `profile_table` | Deep analysis: sample rows, disk usage, inferred relationships |
| `query` | Execute read-only SQL, results as JSON |
| `explain_query` | PostgreSQL execution plans with optional ANALYZE |

Full reference: [isthmus.dev/tools/overview](https://isthmus.dev/tools/overview)

## Documentation

Visit **[isthmus.dev](https://isthmus.dev)** for the full documentation:

- [Installation](https://isthmus.dev/docs/installation) — prebuilt binaries, `go install`, Docker
- [Configuration](https://isthmus.dev/docs/configuration) — env vars, CLI flags, full reference
- [Client setup](https://isthmus.dev/docs/clients/claude-desktop) — Claude Desktop, Cursor, Windsurf, Gemini CLI, VS Code
- [Column masking](https://isthmus.dev/docs/features/column-masking) — PII protection with redact, hash, partial, null
- [Policy engine](https://isthmus.dev/docs/features/policy-engine) — business context, schema filtering
- [Tools reference](https://isthmus.dev/docs/tools/overview) — what each tool does and how the AI uses them

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). You'll need Go 1.25+ and Docker for integration tests.

```bash
make build        # Build binary
make test         # All tests (needs Docker)
make test-short   # Unit tests only
make lint         # Lint
```

## License

[Apache 2.0](LICENSE)
