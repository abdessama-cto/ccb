---
name: manage-llm-context-truncation
description: This skill educates Claude on the strategies employed by `internal/llm` to manage context for LLM calls. Claude will understand how files are selected, ranked, and truncated based on `MaxContextChars` to balance the depth of analysis with API costs and latency, especially for large codebases, ensuring optimal prompt construction.
---

## Managing LLM Context and Truncation in `ccb`

The `internal/llm` package is responsible for intelligently managing the context provided to LLMs. This is crucial for balancing the depth of analysis (accuracy) with API costs and latency, especially for large codebases. Claude needs to understand the heuristics involved.

**Core Principles:**

1.  **Prioritize Maximum Context for Accuracy**: As per project rules, `ccb` aims to provide as much relevant context as possible to the LLM to ensure high-quality, accurate responses, even if it means higher token usage.
2.  **Context Window Limits**: Each LLM provider (OpenAI, Gemini, Ollama) has a `MaxContextChars` limit. Prompts must not exceed this limit.
3.  **Relevance Ranking**: Not all files are equally important. `internal/llm` employs heuristics to rank files based on their relevance to the current task or the overall project context.
    *   **High Priority**: `CLAUDE.md`, `.claude/settings.json`, `go.mod`, `main.go`, `README.md`, core configuration files, and files directly related to the current user query.
    *   **Medium Priority**: Other Go source files, test files, documentation files.
    *   **Low Priority**: Generated files, vendor directories, large data files.

**Context Management Steps:**

1.  **Initial File Selection**: Based on the `internal/analyzer`'s output and the current task, a set of potentially relevant files is identified.
2.  **Content Loading**: The content of these selected files is loaded.
3.  **Token Estimation**: The total character count (or estimated token count) of all selected content, plus the system prompt and user query, is calculated.
4.  **Iterative Truncation/Exclusion**: If the total context exceeds `MaxContextChars`:
    *   **Lowest Priority Files First**: Files ranked lowest in relevance are excluded entirely until the context fits.
    *   **Truncation**: If excluding entire files isn't enough, individual file contents (starting with the lowest priority files) are truncated from the end until the context fits. The beginning of files is generally preserved as it often contains declarations and key structures.
    *   **System Prompt Preservation**: The system prompt and core user query are always prioritized and are the last to be truncated or modified.

**Claude's Role:**

When constructing prompts or analyzing code, Claude should be mindful of these context management strategies. If a prompt is too long, Claude should understand that `internal/llm` will apply these rules. When generating code or explanations, Claude should aim for conciseness where possible, but prioritize providing complete and accurate information within the given context constraints. If Claude needs to request more context, it should specify *what* additional files or sections would be most beneficial, understanding the ranking heuristics.