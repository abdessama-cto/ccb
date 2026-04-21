## Project-Specific Rules for `ccb`

1.  **LLM Output Review**: Always manually review LLM-generated code, commands, or file paths for security vulnerabilities, correctness, and adherence to project conventions before execution or writing to disk.
2.  **GitHub CLI Dependency**: Ensure the `gh` (GitHub CLI) tool is installed and authenticated before running `ccb` commands that interact with GitHub. If `gh` is missing or fails, `ccb` will terminate immediately.
3.  **LLM Context Strategy**: When working with LLM prompts, prioritize maximum context for accuracy, but be mindful of API costs and latency. Leverage the context management heuristics in `internal/llm`.
4.  **File Overwrite Policy**: Be aware that `internal/generator` will always overwrite existing Claude Code configuration files (e.g., `CLAUDE.md`, files in `.claude/`) with newly generated content. Review changes carefully before committing.
5.  **LLM Parsing Robustness**: When designing LLM interactions or parsing their output, always consider the robust JSON sanitization (`sanitizeJSONStrings`) and plain-text file block parser (`scanFileBlocks`) within `internal/llm` to handle formatting inconsistencies.
6.  **TUI Styling Consistency**: All Terminal User Interface (TUI) elements must maintain consistent styling using `charmbracelet/lipgloss` as defined in `internal/tui`.
7.  **Cobra CLI Usage**: New CLI commands and subcommands must follow the `ccb <command> [subcommand]` structure and utilize the `cobra` framework as established in `cmd/ccbootstrap`.
8.  **Configuration Distinction**: Global user configuration is managed via YAML (`~/.ccb/config.yaml`), while project-specific settings are stored in JSON (`.claude/settings.json`). Be mindful of this distinction when modifying configurations.
9.  **Claude Artifact Structure**: All generated Claude Code artifacts (agents, rules, skills, commands, hooks) must be organized within the `.claude/` directory structure.
10. **Go Error Handling**: Follow Go's standard `error` interface for error handling, often wrapping underlying errors for context, as demonstrated throughout the codebase.