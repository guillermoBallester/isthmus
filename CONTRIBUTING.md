# Contributing to Isthmus

Thanks for your interest in contributing! Here's how to get started.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<you>/isthmus.git`
3. Create a branch: `git checkout -b my-change`
4. Make your changes
5. Run checks: `make lint test`
6. Commit and push
7. Open a pull request against `main`

## Prerequisites

- Go 1.24+
- Docker (for integration tests)
- [golangci-lint](https://golangci-lint.run/) (for linting)

## Development Workflow

```bash
make build       # Build binary
make test        # All tests (requires Docker)
make test-short  # Unit tests only
make lint        # Lint
make fmt         # Format code
```

## Pull Request Guidelines

- Keep PRs focused — one logical change per PR
- Add tests for new functionality
- Make sure CI passes (build, test, lint)
- Update the README if your change affects user-facing behavior

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use the existing hexagonal architecture: domain logic in `core/`, infrastructure in `adapter/`
- Keep the core package free of external dependencies

## Reporting Bugs

Open a [GitHub issue](https://github.com/guillermoBallester/isthmus/issues/new) with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Go version and OS

## Security Issues

Please report security vulnerabilities privately. See [SECURITY.md](SECURITY.md) for details.

## License

By contributing, you agree that your contributions will be licensed under the [Apache 2.0 License](LICENSE).
