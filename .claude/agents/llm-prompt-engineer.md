---
name: llm-prompt-engineer
description: When working on features that involve interacting with external LLM providers, especially within the `internal/llm` package, invoke this agent. It can assist in designing, refining, and optimizing prompts to achieve better accuracy, reduce token costs, ensure specific output formats (like the plain-text file block format or sanitized JSON), and manage context effectively for various LLM providers.
tools:
  - read_file
  - write_file
  - shell_command
---
