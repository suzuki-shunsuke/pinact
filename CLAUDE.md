# Project Guidelines for AI Assistants

## Language

This project uses **English** for all code comments, documentation, and communication.

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Common Types

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools
- `ci`: Changes to CI configuration files and scripts

### Examples

```
feat: add GitHub token management via keyring
fix: handle empty configuration file correctly
docs: add function documentation to controller package
chore(deps): update dependency aquaproj/aqua-registry to v4.403.0
```

## Code Validation

After making code changes, **always run** the following commands to validate and test:

### Validation (go vet)

```bash
cmdx v
```
This command runs `go vet ./...` to check for common Go mistakes.

### Testing

```bash
cmdx t
```
This command runs all tests in the project.

Both commands should pass before committing changes.

## Project Structure

```
pinact/
├── cmd/           # Main applications
├── pkg/           # Go packages
│   ├── cli/       # CLI interface layer
│   ├── config/    # Configuration management
│   ├── controller/# Business logic
│   ├── github/    # GitHub API integration
│   └── util/      # Utility functions
├── testdata/      # Test fixtures
├── json-schema/   # JSON schema definitions
└── scripts/       # Build and utility scripts
```

## Testing

- Run all tests: `cmdx t` or `go test ./...`
- Run specific package tests: `go test ./pkg/controller/run`
- Generate coverage: `./scripts/coverage.sh`

## Dependencies

This project uses:
- [aqua](https://aquaproj.github.io/) for tool version management
- [cmdx](https://github.com/suzuki-shunsuke/cmdx) for task runner
- [goreleaser](https://goreleaser.com/) for releases

## Code Style Guidelines

1. Follow standard Go conventions
2. Use meaningful variable and function names
3. Add comments for exported functions and types
4. Keep functions focused and small
5. Handle errors explicitly
6. Use context for cancellation and timeouts
7. Always end files with a newline character

## Pull Request Process

1. Create a feature branch from `main`
2. Make changes and ensure `cmdx v` passes
3. Write clear commit messages following Conventional Commits
4. Create PR with descriptive title and body
5. Wait for CI checks to pass
6. Request review if needed

## Important Commands

```bash
# Validate code (go vet)
cmdx v

# Run tests
cmdx t

# Build the project
go build ./cmd/pinact

# Generate JSON schema
cmdx js

# Run pinact locally
go run ./cmd/pinact run
```

## GitHub Actions Integration

The project includes GitHub Actions for:
- Testing on multiple platforms
- Linting and validation
- Release automation
- Security scanning

## Configuration

pinact supports configuration via:
- `.pinact.yaml` or `.github/pinact.yaml`
- Command-line flags
- Environment variables

## Environment Variables

- `GITHUB_TOKEN`: GitHub access token for API calls
- `PINACT_LOG_LEVEL`: Log level (debug, info, warn, error)
- `PINACT_CONFIG`: Path to configuration file
- `PINACT_KEYRING_ENABLED`: Enable keyring for token storage

## Debugging

Enable debug logging:
```bash
export PINACT_LOG_LEVEL=debug
pinact run
```

## Common Tasks

### Adding a New Command

1. Create new package under `pkg/cli/`
2. Implement command structure with `urfave/cli/v3`
3. Add controller logic under `pkg/controller/`
4. Register command in `pkg/cli/runner.go`
5. Add tests for new functionality

### Updating Schema

1. Modify `pkg/config/config.go`
2. Update JSON schema: `cmdx js`
3. Update documentation in `docs/schema_v*.md`
4. Add migration logic if needed

## Resources

- [Project README](README.md)
- [Contributing Guidelines](CONTRIBUTING.md)
- [Installation Guide](INSTALL.md)
- [Usage Documentation](USAGE.md)
