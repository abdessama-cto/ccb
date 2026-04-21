---
name: parse-llm-file-blocks
description: This skill teaches Claude how to reliably parse plain-text outputs from LLMs that adhere to the `=== FILE: path ===\ncontent\n=== END FILE ===` delimiter format. Claude will learn to extract individual file paths and their corresponding contents, handling potential variations or malformations in the LLM's response, similar to the `scanFileBlocks` function in `internal/llm`.
---

## How to Parse LLM File Blocks

When an LLM generates multiple files in a single plain-text response, `ccb` expects a specific delimiter format to parse them reliably. This skill outlines how to interpret and extract content from this format.

**Format Specification:**

Each file block starts with `=== FILE: <path> ===` and ends with `=== END FILE ===`.

```
=== FILE: path/to/file1.md ===
# Content of File 1
This is the first file's content.
It can span multiple lines.
=== END FILE ===

=== FILE: path/to/another/file2.go ===
package main

func main() {
    // Content of File 2
}
=== END FILE ===
```

**Parsing Steps:**

1.  **Identify Start Delimiter**: Look for the exact string `=== FILE: `.
2.  **Extract Path**: The file path is the string immediately following `=== FILE: ` until the next ` ===` (including the space before `===`).
3.  **Extract Content**: The content of the file starts on the line immediately after the start delimiter and continues until the line *before* the `=== END FILE ===` delimiter.
4.  **Identify End Delimiter**: Look for the exact string `=== END FILE ===`.
5.  **Handle Multiple Blocks**: Repeat steps 1-4 for subsequent blocks in the same response.
6.  **Error Handling**: Be prepared for malformed outputs:
    *   Missing `=== END FILE ===` for a `=== FILE: `.
    *   Incorrect delimiter syntax.
    *   Extra text outside of file blocks (should generally be ignored or flagged).

**Example in `internal/llm/file_block_parser.go`:**

The `scanFileBlocks` function in `internal/llm` implements this parsing logic. It iterates through the LLM response, identifies these delimiters, and returns a map of file paths to their contents. When generating output, Claude should strive to strictly adhere to this format to ensure `ccb` can correctly process the generated files.