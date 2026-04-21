## Code Quality and Security for `ccb` (Go Project)

1.  **Go Best Practices**: Adhere to standard Go idioms and best practices as outlined in "Effective Go" and "Go Code Review Comments".
    *   **Readability**: Write clear, concise, and self-documenting code. Use meaningful variable and function names.
    *   **Simplicity**: Prefer simple solutions over complex ones. Avoid unnecessary abstractions.
    *   **Concurrency**: Use Go's concurrency primitives (`goroutines`, `channels`) correctly and safely. Avoid race conditions.
2.  **Error Handling**: Follow the project convention of using Go's standard `error` interface, often wrapping underlying errors with context using `fmt.Errorf("context: %w", err)`.
    *   Handle errors explicitly; avoid ignoring them.
    *   Return errors up the call stack rather than panicking, except for truly unrecoverable situations.
3.  **Linting and Formatting**: Use `go fmt` for consistent code formatting. Integrate linters (e.g., `golangci-lint`) to catch common issues, potential bugs, and style violations.
4.  **Security**: 
    *   **Input Validation**: Validate all user inputs and external API responses, especially before using them in file paths, shell commands, or database queries.
    *   **Sensitive Data**: Never hardcode API keys, tokens, or other sensitive information. Use environment variables or secure configuration management.
    *   **LLM Output Sanitization**: Strictly enforce the sanitization rules for LLM-generated content, particularly for JSON and potential shell commands, as implemented in `internal/llm`.
    *   **Dependency Vulnerabilities**: Regularly check for vulnerabilities in third-party Go modules using tools like `go list -m all` and `govulncheck`.
5.  **Performance**: Be mindful of performance, especially in critical loops or when processing large amounts of data (e.g., static code analysis). Profile code where necessary.
6.  **Comments**: Write comments to explain *why* code does something, not just *what* it does, especially for complex algorithms, non-obvious logic, or public API functions.
7.  **Avoid Global State**: Minimize the use of global variables. Pass dependencies explicitly to functions and structs.