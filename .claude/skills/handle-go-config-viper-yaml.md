---
name: handle-go-config-viper-yaml
description: Explains how to load, save, and migrate global user configuration from YAML files (e.g., `~/.ccb/config.yaml`) using Go's configuration libraries.
---

## Handling Go Configuration with YAML and Migration Logic

`ccb` manages its global user configuration primarily through YAML files, specifically `~/.ccb/config.yaml`. The `internal/config` package is responsible for loading, saving, and migrating these settings. Claude needs to understand the structure and process for configuration management.

**1. Configuration File Location and Format:**

*   **Global User Config**: `~/.ccb/config.yaml`
    *   **Format**: YAML
    *   **Content Example**:
        ```yaml
        llm:
          provider: openai
          openai_api_key: sk-...
          default_model: gpt-4o
        ui:
          theme: dark
          verbose_logging: false
        ```
*   **Project-Specific Settings**: `.claude/settings.json`
    *   **Format**: JSON
    *   **Content Example**:
        ```json
        {
          "deny_patterns": [
            "/tmp/*",
            "/etc/*"
          ]
        }
        ```

**2. Loading Configuration:**

*   The `internal/config` package typically uses a library like `spf13/viper` or custom unmarshalling logic to load the YAML file into a Go struct.
*   It first checks for the existence of the config file. If not found, it might create a default configuration.
*   Environment variables are often used to override configuration values (e.g., `CCB_OPENAI_API_KEY`).

**3. Saving Configuration:**

*   After modifications (e.g., user changes LLM provider via `ccb config set`), the Go config struct is marshaled back into YAML and written to `~/.ccb/config.yaml`.
*   Permissions on the config file should be set securely (e.g., `0600`).

**4. Configuration Migration:**

`ccb` includes logic to handle migrations from older configuration formats or locations (e.g., `.ccbootstrap` to `.ccb`).

*   **Detection**: The migration logic first checks for the existence of legacy config files or directories.
*   **Conversion**: If legacy config is found, it's read, converted to the new format, and written to the new location.
*   **Cleanup**: Optionally, the old config files might be backed up or removed.
*   **Example**: Migrating `~/.ccbootstrap/config.yaml` to `~/.ccb/config.yaml`.

**Claude's Task:**

When working with configuration, Claude should:
*   Understand the distinction between global YAML config and project-specific JSON settings.
*   Know where to find and how to interpret `~/.ccb/config.yaml`.
*   When proposing changes to configuration, ensure they adhere to the YAML format and the expected structure of the `ccb` config.
*   Be aware of the migration logic and consider its implications when suggesting changes to config file paths or formats.