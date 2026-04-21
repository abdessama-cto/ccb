---
name: use-cobra-cli-framework
description: This skill teaches Claude how to effectively use the `spf13/cobra` framework for building Go CLI applications. Claude will learn to define root commands, subcommands, persistent and local flags, and argument parsing, ensuring new `ccb` commands adhere to the project's established CLI structure.
---

## Using the `spf13/cobra` CLI Framework

`ccb` is built as a Go CLI application using `spf13/cobra` for command-line parsing. Claude needs to understand how to define and structure commands, subcommands, and flags using this framework, as demonstrated in `cmd/ccbootstrap`.

**Core Concepts:**

1.  **`cobra.Command`**: The fundamental building block. Each command (e.g., `ccb`, `ccb init`, `ccb update`) is an instance of `cobra.Command`.

2.  **Root Command**: The main entry point of the application (e.g., `ccb`). All other commands are subcommands of the root.
    *   Defined in `cmd/ccbootstrap/root.go`.
    *   Typically has a `PersistentPreRunE` or `PersistentPostRunE` for global setup/teardown.

3.  **Subcommands**: Commands nested under other commands (e.g., `init` is a subcommand of `ccb`).
    *   Added using `rootCmd.AddCommand(initCmd)`.

4.  **`RunE` Function**: The core logic of a command. This function is executed when the command is invoked.
    *   It takes `*cobra.Command` and `[]string` (arguments) as input and returns an `error`.
    *   `RunE` is preferred over `Run` because it allows returning errors directly.

5.  **Flags**: Options that modify a command's behavior.
    *   **Persistent Flags**: Apply to the command and all its subcommands (e.g., `--verbose`, `--config`).
        *   Defined using `cmd.PersistentFlags().BoolVarP(...)`.
    *   **Local Flags**: Apply only to the command they are defined on.
        *   Defined using `cmd.Flags().StringVarP(...)`.
    *   **Required Flags**: Can be marked as required using `cmd.MarkFlagRequired("flag-name")`.

6.  **Arguments**: Non-flag inputs to a command (e.g., `ccb init <repo_path>`).
    *   Accessed via the `args []string` parameter in `RunE`.
    *   `cobra` provides validators like `cobra.ExactArgs(1)`, `cobra.MinimumNArgs(1)` to enforce argument counts.

**Example Structure (simplified):**

```go
// cmd/ccbootstrap/root.go
var rootCmd = &cobra.Command{
    Use:   "ccb",
    Short: "Claude Code Bootstrapper CLI",
    Long:  `A CLI tool to bootstrap Claude Code configurations...`,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Global setup, e.g., load config
        return nil
    },
}

// cmd/ccbootstrap/init.go
var initCmd = &cobra.Command{
    Use:   "init [repo_path]",
    Short: "Initialize Claude Code for a project",
    Args:  cobra.ExactArgs(1), // Expects exactly one argument
    RunE: func(cmd *cobra.Command, args []string) error {
        repoPath := args[0]
        // Call internal logic, e.g., internal/analyzer.Analyze(repoPath)
        return nil
    },
}

func init() {
    rootCmd.AddCommand(initCmd)
    initCmd.Flags().BoolVarP(&forceOverwrite, "force", "f", false, "Force overwrite existing files")
}
```

**Claude's Task:**

When asked to add a new CLI command or modify an existing one, Claude should:
*   Determine if it's a top-level command or a subcommand.
*   Define the `cobra.Command` struct with `Use`, `Short`, `Long`, and `RunE`.
*   Add appropriate `Args` validators.
*   Define any necessary `PersistentFlags` or `Flags`.
*   Ensure the command is added to its parent command (e.g., `rootCmd.AddCommand(newCmd)`).
*   Adhere to the `ccb <command> [subcommand]` naming convention.