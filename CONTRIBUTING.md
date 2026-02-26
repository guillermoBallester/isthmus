# Contributing to Isthmus

Thanks for your interest in contributing to Isthmus!

## Getting started

1. Fork the repository
2. Clone your fork
3. Create a branch for your change

```bash
git checkout -b my-feature
```

## Development setup

You need Go 1.25+ and Docker (for integration tests).

```bash
make build        # Build the binary
make test-short   # Run unit tests (no Docker needed)
make test         # Run all tests (needs Docker for testcontainers)
make lint         # Run linter
```

## Making changes

- Follow existing code conventions — hexagonal architecture, no global state, no mocks
- All SQL validation changes need test cases in `validator_test.go`
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Keep SQL queries in `internal/adapter/postgres/queries.go`

## Submitting a pull request

1. Make sure all tests pass: `make test`
2. Run the linter: `make lint`
3. Push your branch and open a PR against `main`
4. Describe what your change does and why

## Reporting bugs

Open a GitHub issue with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Your environment (OS, Go version, Postgres version)

## Security vulnerabilities

Do **not** open a public issue. See [SECURITY.md](SECURITY.md) for responsible disclosure instructions.
