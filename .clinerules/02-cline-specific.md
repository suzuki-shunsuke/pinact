# Cline-Specific Guidelines

## VS Code Integration

- Leverage VS Code's built-in features for file navigation and editing
- Use the integrated terminal for command execution
- Respect workspace settings and configurations

## File Operations

- Ensure UTF-8 encoding for all text files
- Use relative paths when possible for portability
- Always check file existence before operations

## Testing and Validation

Before committing any changes:
1. Run `cmdx v` for code validation (go vet)
2. Run `cmdx t` to execute all tests
3. Ensure all checks pass

## Error Reporting

When reporting errors:
- Include file paths with line numbers (e.g., `pkg/controller/run/parse_line.go:123`)
- Provide clear, actionable error messages
- Use markdown formatting for better readability

## Git Operations

- Use conventional commit messages (see [AI_GUIDE.md](../AI_GUIDE.md#commit-messages))
- Create feature branches from `main`
- Write descriptive PR titles and bodies

## Resource Management

- Be mindful of file system operations
- Clean up temporary files if created
- Don't modify files outside the project directory