## Testing Expectations for `ccb` (Go Project)

1.  **Unit Tests**: Every new function or significant code block should be accompanied by unit tests. Tests should be granular, fast, and cover expected behavior, edge cases, and error conditions.
    *   Use Go's built-in `testing` package.
    *   Place test files in the same directory as the code they test, with a `_test.go` suffix (e.g., `analyzer.go` -> `analyzer_test.go`).
    *   Aim for high code coverage, especially for core logic in `internal/llm`, `internal/analyzer`, and `internal/generator`.
2.  **Integration Tests**: For modules interacting with external services or complex internal flows (e.g., `internal/github`, `internal/llm`, `internal/skills`), write integration tests. These tests may involve mocking external APIs or using test doubles to simulate dependencies.
    *   Ensure `internal/llm`'s prompt building and response parsing logic is thoroughly tested against various LLM output scenarios.
    *   Test the `internal/generator`'s ability to correctly write files to a temporary directory.
3.  **CLI Tests**: Test the `cmd/ccbootstrap` commands end-to-end. This involves simulating command-line arguments and verifying output and side effects.
    *   Use `os/exec` to run `ccb` commands in tests.
    *   Verify TUI interactions where possible, or test the underlying logic that drives the TUI.
4.  **TUI Component Tests**: While full TUI interaction testing can be complex, ensure the underlying models and update logic for `charmbracelet/bubbletea` components are well-tested.
5.  **Test Data**: Use realistic but anonymized test data. Avoid using live API keys or sensitive information in tests.
6.  **Continuous Integration (CI)**: Although not currently configured, all tests should pass before any code is merged to `main`. When CI is introduced, it will automatically run all tests on every push to a PR branch.