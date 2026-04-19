# Code Quality Rules

## Language: Go

## Principles
- Keep functions small and focused (< 50 lines)
- Avoid deeply nested conditionals (max 3 levels)
- Prefer explicit over implicit
- No commented-out code in PRs
- Remove unused imports and variables

## Security
- Never hardcode secrets or API keys
- Use environment variables for configuration
- Validate all user inputs
- Use parameterized queries for database access
