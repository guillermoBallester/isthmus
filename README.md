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

<p align="center">
  <img src="docs/assets/demo.gif" alt="Isthmus demo" width="700" />
</p>

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
    Claude["Claude Desktop"] & Cursor["Cursor / VS Code"] -->|stdio| STDIO
    ChatGPT["ChatGPT / Web"] -->|HTTP| HTTP

    subgraph Transport["Transport"]
        STDIO["stdio"]
        HTTP["HTTP + Auth"]
    end

    STDIO & HTTP --> Router

    subgraph Tools["MCP Tools"]
        Router{{"router"}}
        Router --> Discover["discover"]
        Router --> Describe["describe_table"]
        Router --> Query["query"]
    end

    Discover & Describe --> Explorer

    subgraph Schema["Schema Explorer"]
        Explorer["Catalog Introspection"]
        Explorer --> Policy["Policy Engine"]
    end

    Query --> Validate

    subgraph Security["Security Pipeline"]
        direction TB
        Validate["AST Validation"] --> ReadOnly["Read-Only Tx"]
        ReadOnly --> RowLimit["Row Limit"]
        RowLimit --> Timeout["Timeout"]
    end

    Security --> PG[("PostgreSQL")]
    Schema --> PG

    PG --> Mask

    subgraph Post["Post-Processing"]
        direction TB
        Mask["PII Masking"] --> Sanitize["Error Sanitization"]
    end

    Post -.-> Audit["Audit Log"]
    Post -.-> OTel["OpenTelemetry"]
    Post --> Response["Safe Response"]
    Response --> Claude & Cursor & ChatGPT

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
    class Validate,ReadOnly,RowLimit,Timeout security
    class Explorer,Policy explorer
    class Mask,Sanitize postproc
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
