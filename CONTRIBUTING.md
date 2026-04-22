# Contributing

Hey, thanks for wanting to contribute to `storage-go`! These guidelines will help make sure that your contribution helps us make `storage-go` as good as it can possibly be.

We are looking for the following contributions:

- Minor functionality
- Bugfixes
- Bug and usage experience reports
- Writing tutorials and guides
- Documentation improvements

Please [join the Discord](https://community.tigrisdata.com/) before contributing so that we can help make sure that bugs are in fact bugs or that any code contributions would be useful for the whole community. Please join the `#storage-go` channel to talk with us!

## Finding Issues to Work On

Check out the [GitHub Issues](https://github.com/tigrisdata/storage-go/issues) for open items. Issues labeled `good first issue` are great starting points for new contributors.

## Development Setup

1. Fork the repository on GitHub
2. Clone your fork locally:

   ```bash
   git clone https://github.com/YOUR_USERNAME/storage-go.git
   cd storage-go
   ```

3. Install Go dependencies:

   ```bash
   go mod download
   ```

4. Install JavaScript tooling dependencies:

   ```bash
   npm install
   ```

## Development Workflow

### Building

```bash
go build ./...
```

### Running Tests

```bash
npm test
# or
go test ./...
```

### Code Formatting

Before committing, run the formatter to ensure your code follows the project's style:

```bash
npm run format
```

This runs:

- `goimports` for Go code (tabs, `camelCase` variables, `PascalCase` exports)
- `prettier` for JSON/YAML/JS (2-space indentation, double quotes, trailing commas)

## Code Style Guidelines

### Go

- Follow standard library style
- Prefer table-driven tests
- Use `goimports` for import management
- Tabs for indentation

### JSON/YAML/JavaScript

- Formatted with Prettier (run `npm run format`)
- 2-space indentation
- Double quotes
- Trailing commas where applicable

## Commit Messages

We follow the [Conventional Commits](https://www.conventionalcommits.org/) format:

```text
[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

Examples:

- `feat(storage): add bucket snapshot support`
- `fix(client): handle timeout errors gracefully`
- `docs: update README with new examples`

### DCO Signoff Requirement

All commits MUST be signed off. Use the `--signoff` flag:

```bash
git commit -m "feat: add new feature" --signoff
```

This is enforced by CI checks.

## Pull Request Process

1. Create a new branch for your work:

   ```bash
   git checkout -b my-feature-branch
   ```

2. Make your changes and commit them with descriptive messages and `--signoff`

3. Push to your fork:

   ```bash
   git push origin my-feature-branch
   ```

4. Open a pull request on GitHub
   - Use the PR template (`.github/pull_request_template.md`)
   - Reference any related issues in the description
   - Ensure CI passes before requesting review

5. Respond to reviewer feedback and make requested changes

## Reporting Bugs

When reporting bugs, please include:

- **Reproduction steps** – What did you do?
- **Expected behavior** – What did you expect to happen?
- **Actual behavior** – What happened instead?
- **Environment details**
  - Go version (`go version`)
  - OS and architecture
  - Relevant code snippets or minimal reproduction case
- **Error messages** – Full stack traces if available

## Use of AI Tools

We allow the use of AI tools to author contributions with the following stipulations:

- All commits created by or with significant assistance from generative AI tools **MUST** be annotated with the name of the tool being used with the [`Assisted-By` footer](https://xeiaso.net/notes/2025/assisted-by-footer/):

  ```text
  Assisted-by: GLM 4.7 via Claude Code
  ```

- All AI generated code, documentation, and other artifacts **MUST** be reviewed and understood entirely before being submitted upstream.

We also kindly request that you not use AI tooling in such a way that requires us to have more rules.

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
