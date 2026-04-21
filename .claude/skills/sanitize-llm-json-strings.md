---
name: sanitize-llm-json-strings
description: This skill instructs Claude on the critical importance of sanitizing LLM-generated JSON strings to ensure they are valid and safe for programmatic use. Claude will understand common issues like unescaped characters, malformed structures, and how to apply robust sanitization techniques, mirroring the logic found in `internal/llm/json_sanitizer.go`.
---

## Sanitizing LLM-Generated JSON Strings

LLMs can sometimes produce JSON that is syntactically incorrect or contains unescaped characters, leading to parsing errors in Go. The `internal/llm` package, specifically `json_sanitizer.go`, contains robust logic to handle these issues. Claude must understand these common pitfalls and the sanitization strategies.

**Common Issues with LLM JSON Output:**

1.  **Unescaped Newlines/Tabs**: LLMs might include raw `\n` or `\t` characters within string values without proper JSON escaping (`\\n`, `\\t`).
2.  **Unescaped Quotes**: Double quotes (`"`) within a string value that are not escaped (`\"`).
3.  **Trailing Commas**: Commas after the last element in an array or object.
4.  **Comments**: LLMs might include `//` or `/* */` style comments, which are not valid JSON.
5.  **Invalid Unicode Characters**: Characters that are not valid in JSON strings or are improperly encoded.
6.  **Incomplete JSON**: Truncated responses or missing closing braces/brackets.

**Sanitization Strategy (as implemented in `internal/llm/json_sanitizer.go`):**

The `sanitizeJSONStrings` function typically employs a multi-step approach:

1.  **Initial Regex Replacements**: Use regular expressions to replace common unescaped characters (e.g., `\n`, `\t`, `\r`) with their properly escaped JSON equivalents (`\\n`, `\\t`, `\\r`).
2.  **Quote Escaping**: Identify and escape unescaped double quotes within string values that would otherwise break the JSON structure.
3.  **Comment Removal**: Strip out any `//` or `/* */` comments.
4.  **Trailing Comma Removal**: Use regex or a custom parser to remove trailing commas from arrays and objects.
5.  **Attempt Parsing and Re-encoding**: A common robust technique is to attempt to unmarshal the (partially) sanitized JSON into a generic `map[string]interface{}` or `[]interface{}`, and then re-marshal it. This process automatically corrects many structural issues and ensures valid escaping.
    *   If the initial unmarshalling fails, further iterative sanitization or more aggressive parsing might be attempted.

**Claude's Role:**

When generating JSON, Claude should strive to produce perfectly valid JSON from the outset. However, if Claude is tasked with processing or reviewing LLM output that is expected to be JSON, it should be aware that `internal/llm` will apply these sanitization steps. Claude should anticipate these corrections and understand that the final parsed JSON might differ slightly from the raw LLM output due to these necessary security and correctness measures.