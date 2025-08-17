# Project Guidelines for pinact

For comprehensive project guidelines, please refer to [AI_GUIDE.md](../AI_GUIDE.md).

## Core References

- **General Guidelines**: See [AI_GUIDE.md](../AI_GUIDE.md) for:
  - Language conventions (English only)
  - Commit message format (Conventional Commits)
  - Code validation and testing
  - Project structure
  - Package responsibilities
  - Error handling patterns

## Quick Command Reference

```bash
# Validate code (go vet)
cmdx v

# Run tests
cmdx t

# Generate JSON schema
cmdx js

# Build the project
go build ./cmd/pinact
```

## Development Workflow

1. Review task requirements
2. Make code changes following Go conventions
3. Validate with `cmdx v`
4. Run tests with `cmdx t`
5. Commit with conventional commit messages
6. Create pull requests with descriptive titles and bodies

## Important Notes

- Always end files with a newline character
- Use meaningful variable and function names
- Add comments for exported functions and types
- Handle errors explicitly
- Keep functions focused and small