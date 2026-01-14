# Repository Guidelines

## Project Structure & Module Organization

```text
├─ storage.go                 # Core package with client factory and configuration
├─ client.go                  # Main client wrapper with Tigris-specific methods
├─ tigrisheaders/             # Tigris-specific HTTP header utilities
├─ simplestorage/             # Simplified storage client wrapper
├─ .github/workflows/         # CI/CD workflows
├─ test files                 # Go test files live alongside source (`*_test.go`)
├─ example_test.go            # Godoc examples
├─ package.json               # npm scripts for dev tooling
└─ go.mod                     # Go module definition
```

This is a Go SDK library for Tigris Data storage, not a binary application. Source code is primarily Go; JavaScript tooling lives under `node_modules` and the root `package.json`.

## Development Workflow

### Build, Test & Development Commands

| Command          | Description                                        |
| ---------------- | -------------------------------------------------- |
| `npm test`       | Runs tests with `go test ./...`.                   |
| `go build ./...` | Compiles all Go packages.                          |
| `go test ./...`  | Runs all tests.                                    |
| `npm run format` | Formats Go (`goimports`) and JS/HTML (`prettier`). |

### Code Formatting & Style

- **Go** – use `go fmt`/`goimports`; tabs for indentation, `camelCase` for variables, `PascalCase` for exported identifiers
- **JavaScript/JSON/YAML** – formatted with Prettier (2-space tabs, trailing commas, double quotes)
- Files are `snake_case`; packages use lower-case module names
- Run `npm run format` before committing

### Testing

- Tests are written in Go using the standard `testing` package (`*_test.go`)
- Keep test files next to the code they cover
- Run the full suite with `npm test` or `go test ./...`
- Package examples are in `example_test.go` for Godoc

## Code Quality & Security

### Commit Guidelines

Commit messages follow **Conventional Commits** format:

```text
[optional scope]: <description>
[optional body]
[optional footer(s)]
```

**Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

- Add `!` after type/scope for breaking changes or include `BREAKING CHANGE:` in the footer.
- Keep descriptions concise, imperative, lowercase, and without a trailing period.
- Reference issues/PRs in the footer when applicable.
- **ALL git commits MUST be made with `--signoff`.** This is enforced by DCO checks.

### Attribution Requirements

AI agents must disclose what tool and model they are using in the "Assisted-by" commit footer:

```text
Assisted-by: [Model Name] via [Tool Name]
```

Example:

```text
Assisted-by: GLM 4.7 via Claude Code
```

## Helpful Documentation

When asked about various services or tools, use these resources to help you:

- **Tigris** or **Tigris Data**: https://www.tigrisdata.com/docs/llms.txt

## Pull Request Requirements

- **ALL pull requests MUST use the template in `.github/pull_request_template.md`**
- Include a clear description of changes
- Reference any related issues
- Pass CI (`npm test`)
- PR titles must follow Conventional Commits format (enforced by linting)

### Security Best Practices

- Secrets never belong in the repo; use environment variables
- Run `npm audit` periodically for JS tooling vulnerabilities

## AI Assistant Instructions

### Task Execution

When undertaking a task, pause and ask the user for intent before writing code.

### Technical Guidelines

- **Go** – follow the standard library style; prefer table-driven tests
- **JSON/YAML** – double quotes, two-space indentation
- Run `npm run format` to apply Prettier and goimports formatting

## Implementation Details

### Project Architecture

This is an SDK library wrapping the AWS S3 client with Tigris-specific enhancements:

1. **Main Package** (`storage.go`, `client.go`)
   - Entry point: `New(ctx context.Context, options ...Option) (*Client, error)`
   - Functional options pattern for configuration
   - Tigris-specific methods: bucket snapshots, forks, in-place object renaming

2. **tigrisheaders Package**
   - Lower-level utilities for setting Tigris-specific HTTP headers
   - Region constants for static replication (FRA, GRU, HKG, IAD, etc.)
   - Query filtering, conditional operations, snapshot versioning

3. **simplestorage Package**
   - Simplified abstraction with environment variable configuration
   - Helper wrapper for common use cases

### Key Design Patterns

- **Wrapper Pattern**: Client wraps AWS SDK S3 client
- **Functional Options**: Used for client configuration
- **Separation of Concerns**: HTTP headers in separate package

### Dependencies

- AWS SDK Go v2 (`github.com/aws/aws-sdk-go-v2/service/s3`)
- Default endpoint: `https://fly.storage.tigris.dev` (Fly.io optimized)

---

_This file is consulted by the repository's tooling. Keep it up-to-date as the project evolves._
