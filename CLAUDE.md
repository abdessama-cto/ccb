This project, `ccb` (Claude Code Bootstrapper), is a macOS native Go CLI tool designed to automate the setup of Claude Code configurations for any GitHub repository or local project. It analyzes codebases, guides users through an AI-driven questionnaire, and generates tailored Claude Code files (CLAUDE.md, agents, rules, skills, documentation), significantly reducing setup time for AI assistance in development workflows.

## Stack
- **Primary Language**: Go
- **CLI Framework**: `spf13/cobra`
- **Terminal UI**: `charmbracelet/bubbletea`, `charmbracelet/lipgloss`
- **External LLM Integration**: OpenAI, Google Gemini, Ollama (via HTTP APIs)
- **GitHub Interaction**: `gh` CLI (GitHub CLI)
- **Configuration**: YAML (global), JSON (project-specific)

## Key Modules
- `internal/llm`: Centralized logic for all LLM interactions, including prompt building, response parsing, and robust JSON/text sanitization. **This module is critical for all AI-driven features.**
- `internal/analyzer`: Performs static code analysis to extract project context for AI processing.
- `internal/generator`: Responsible for writing the final Claude Code configuration files to disk.
- `internal/github`: Handles all interactions with GitHub, primarily through the `gh` CLI.
- `internal/tui`: Provides reusable and consistently styled Terminal User Interface components.
- `internal/config`: Manages global user configuration and project-specific settings.
- `internal/cache`: Caches AI-generated project analysis results.
- `internal/skills`: Interacts with the `skills.sh` API for skill discovery and fetching.
- `cmd/ccbootstrap`: Defines all CLI commands and their execution logic.

## Project Conventions
- **CLI Structure**: Commands follow `ccb <command> [subcommand]` using `cobra`.
- **Configuration Files**: Global settings in `~/.ccb/config.yaml` (YAML), project-specific settings in `.claude/settings.json` (JSON).
- **Claude Code Artifacts**: Organized within `.claude/` with subdirectories like `agents/`, `rules/`, `skills/`, `commands/`, `hooks/`.
- **LLM Output Format**: Plain-text file blocks use `=== FILE: path ===\ncontent\n=== END FILE ===` delimiters for generation.
- **Error Handling**: Uses Go's standard `error` interface, often wrapping errors for context.
- **Terminal UI Styling**: Consistently styled using `charmbracelet/lipgloss`.
- **Generated Filenames**: Kebab-case with `.md` extension (e.g., `my-agent.md`).

## Strict Rules
- **LLM Output Review**: Always manually review LLM-generated code, commands, or file paths for security vulnerabilities, correctness, and adherence to project conventions before execution or writing to disk.
- **GitHub CLI Dependency**: Ensure the `gh` (GitHub CLI) tool is installed and authenticated before running `ccb` commands that interact with GitHub. If `gh` is missing or fails, `ccb` will terminate immediately.
- **LLM Context Strategy**: When working with LLM prompts, prioritize maximum context for accuracy, but be mindful of API costs and latency. Leverage the context management heuristics in `internal/llm`.
- **File Overwrite Policy**: Be aware that `internal/generator` will always overwrite existing Claude Code configuration files (e.g., `CLAUDE.md`, files in `.claude/`) with newly generated content. Review changes carefully before committing.
- **LLM Parsing Robustness**: When designing LLM interactions or parsing their output, always consider the robust JSON sanitization (`sanitizeJSONStrings`) and plain-text file block parser (`scanFileBlocks`) within `internal/llm` to handle formatting inconsistencies.
- **TUI Styling Consistency**: All Terminal User Interface (TUI) elements must maintain consistent styling using `charmbracelet/lipgloss` as defined in `internal/tui`.
- **Cobra CLI Usage**: New CLI commands and subcommands must follow the `ccb <command> [subcommand]` structure and utilize the `cobra` framework as established in `cmd/ccbootstrap`.
- **Configuration Distinction**: Global user configuration is managed via YAML (`~/.ccb/config.yaml`), while project-specific settings are stored in JSON (`.claude/settings.json`). Be mindful of this distinction when modifying configurations.
- **Claude Artifact Structure**: All generated Claude Code artifacts (agents, rules, skills, commands, hooks) must be organized within the `.claude/` directory structure.
- **Go Error Handling**: Follow Go's standard `error` interface for error handling, often wrapping underlying errors for context, as demonstrated throughout the codebase.