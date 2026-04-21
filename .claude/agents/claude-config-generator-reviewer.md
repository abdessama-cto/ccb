---
name: claude-config-generator-reviewer
description: Before `internal/generator` writes any Claude Code configuration files (e.g., `CLAUDE.md`, agents, rules, skills, documentation) to disk, especially when existing files might be overwritten, invoke this agent. It will review the proposed content for correctness, completeness, adherence to project conventions, and ensure it accurately reflects the project's needs and the user's wizard answers, preventing unintended configurations or data loss.
tools:
  - read_file
  - write_file
---
