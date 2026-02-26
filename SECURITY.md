# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Isthmus, **please do not open a public issue.**

Instead, report it privately via [GitHub Security Advisories](https://github.com/guillermoBallester/isthmus/security/advisories/new).

### What to include

- A description of the vulnerability
- Steps to reproduce (or a proof-of-concept)
- Impact assessment if possible

### Response timeline

- **Acknowledgement**: within 48 hours
- **Initial assessment**: within 7 days
- **Fix or mitigation**: best effort, typically within 30 days for confirmed issues

### Scope

The following are in scope:
- SQL injection or read-only bypass in the query validation layer
- MCP protocol handling vulnerabilities
- Credential leakage through logs, errors, or MCP responses
- Container image vulnerabilities (Dockerfile)

The following are **out of scope**:
- Vulnerabilities in PostgreSQL itself
- Issues requiring physical access to the machine running Isthmus
- Social engineering

## Security Design

See the [Safety & Security](README.md#safety--security) section of the README for details on Isthmus's defense-in-depth approach.
