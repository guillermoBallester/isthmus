<p align="center">
<strong>Isthmus</strong>
</p>

<p align="center">
  <strong>Your database, understood by AI.</strong>
</p>

<p align="center">
  <a href="https://github.com/guillermoBallester/isthmus/actions/workflows/ci.yml"><img src="https://github.com/guillermoBallester/isthmus/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go" /></a>
  <a href="https://www.postgresql.org"><img src="https://img.shields.io/badge/PostgreSQL-12+-4169E1?logo=postgresql&logoColor=white" alt="PostgreSQL" /></a>
  <a href="https://modelcontextprotocol.io"><img src="https://img.shields.io/badge/MCP-2025--03--26-blueviolet" alt="MCP" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License" /></a>
</p>

<p align="center">
  <a href="https://isthmus.dev">Docs</a> &middot;
  <a href="https://isthmus.dev/quickstart">Quickstart</a> &middot;
  <a href="https://isthmus.dev/installation">Install</a> &middot;
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

See the [quickstart guide](https://isthmus.dev/quickstart) for step-by-step setup with Claude Desktop, Cursor, Windsurf, and more.

## Features

- **Schema discovery** — explore schemas, tables, columns, foreign keys, and indexes ([docs](https://isthmus.dev/tools/overview))
- **Read-only queries** — execute SQL with server-side row limits and query timeouts ([docs](https://isthmus.dev/tools/query))
- **Table profiler** — column statistics, cardinality, sample rows, index usage ([docs](https://isthmus.dev/tools/profile-table))
- **Policy engine** — enrich your schema with business context so the AI writes better SQL ([docs](https://isthmus.dev/features/policy-engine))
- **SQL validation** — AST-level whitelist via `pg_query` parser — only `SELECT` and `EXPLAIN` allowed ([docs](https://isthmus.dev/configuration))
- **Works with any MCP client** — Claude Desktop, Cursor, Windsurf, Gemini CLI, VS Code ([client setup](https://isthmus.dev/clients/claude-desktop))

## How it works

```
  Claude / Cursor / VS Code
       │
       │ stdio (MCP)
       ▼
  ┌──────────┐         ┌────────────┐
  │ isthmus  │────────▶│ PostgreSQL │
  └──────────┘         └────────────┘
    your machine          any host
```

Isthmus sits between your AI client and your database. It validates every query at the AST level, enforces read-only transactions, and injects server-side row limits — independent of whatever SQL the LLM generates.

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

- [Installation](https://isthmus.dev/installation) — prebuilt binaries, `go install`, Docker
- [Configuration](https://isthmus.dev/configuration) — env vars, CLI flags, full reference
- [Client setup](https://isthmus.dev/clients/claude-desktop) — Claude Desktop, Cursor, Windsurf, Gemini CLI, VS Code
- [Policy engine](https://isthmus.dev/features/policy-engine) — business context, schema filtering
- [Tools reference](https://isthmus.dev/tools/overview) — what each tool does and how the AI uses them

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
