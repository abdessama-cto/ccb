---
name: github-cli-debugger
description: If `ccb` encounters any errors related to GitHub operations, such as cloning repositories, authenticating with GitHub, or creating pull requests, and the error message suggests an issue with the `gh` (GitHub CLI) tool, invoke this agent. It can help diagnose `gh` installation problems, authentication failures, or incorrect command usage, guiding the user or Claude on how to resolve these dependencies.
tools:
  - shell_command
  - read_file
---
