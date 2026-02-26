# Security Policy

## Architecture

Isthmus is a **local-only** MCP server. Your database credentials and query
results never leave your machine. The binary runs as a stdio process inside
your MCP client (Claude Desktop, Cursor, VS Code).

## Safety features

- **Read-only transactions** — queries run inside `SET TRANSACTION READ ONLY`
- **SQL validation** — only `SELECT` and `EXPLAIN` statements allowed (AST-level via pg_query)
- **Row limits** — server-side `LIMIT` injection on all queries
- **Query timeouts** — `context.WithTimeout` on every execution
- **Schema filtering** — restrict visibility to specific schemas via allowlist
- **Policy engine** — table/column filtering and business context via YAML config
- **No telemetry** — no data is collected or sent anywhere

## Supply chain security

- All GitHub Actions are pinned to full commit SHAs
- Dependencies are monitored via Dependabot (weekly)
- Trivy scans run on every push and weekly (filesystem + container image)
- `govulncheck` checks for known Go vulnerabilities
- CodeQL static analysis on every push

## Reporting a vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Use [GitHub private vulnerability reporting](https://github.com/guillermoBallester/isthmus/security/advisories/new) or email the maintainers directly
3. Include steps to reproduce the vulnerability
4. You can expect an initial response within 48 hours

## Supported versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |
