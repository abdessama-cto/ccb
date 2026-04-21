---
name: perform-static-code-analysis
description: Describes how `internal/analyzer` performs static code analysis (e.g., stack, LOC, tests, CI/CD, Docker, .env detection) to extract semantic context for AI processing.
---

## Performing Static Code Analysis in `ccb`

The `internal/analyzer` package is crucial for `ccb`'s ability to understand a target codebase without requiring an LLM. It performs static code analysis to extract key characteristics and semantic context, which then informs the AI-driven configuration generation. Claude needs to understand the types of analysis performed.

**Objectives of Static Analysis:**

1.  **Identify Project Stack**: Determine the primary programming language(s) and frameworks used.
2.  **Quantify Codebase Size**: Measure Lines of Code (LOC) to estimate project scale and LLM context needs.
3.  **Detect Development Practices**: Identify the presence of tests, CI/CD configurations, Docker setups, and environment variable files.
4.  **Extract Semantic Context**: Identify key directories, module names, and other structural elements that provide meaningful context to an LLM.

**Analysis Steps and Data Points Collected:**

1.  **File System Traversal**: Recursively walk the target project directory, respecting `.gitignore` rules.

2.  **Language Detection**: 
    *   **Go**: Presence of `go.mod`, `.go` files.
    *   **Other**: Detect common file extensions and manifest files for other languages (e.g., `package.json` for Node.js, `pom.xml` for Java, `requirements.txt` for Python).

3.  **Lines of Code (LOC)**:
    *   Count physical lines of code for relevant source files.
    *   Distinguish between source code, test code, and documentation.

4.  **Test Detection**: 
    *   **Go**: Presence of `_test.go` files, `go test` commands in scripts.
    *   **General**: Look for `test/`, `spec/` directories, common test runner configurations.

5.  **CI/CD Detection**: 
    *   Look for common CI/CD configuration files:
        *   `.github/workflows/` (GitHub Actions)
        *   `.gitlab-ci.yml` (GitLab CI)
        *   `jenkinsfile` (Jenkins)
        *   `.travis.yml` (Travis CI)

6.  **Containerization Detection**: 
    *   Presence of `Dockerfile`, `docker-compose.yml`.

7.  **Environment Variable Files**: 
    *   Presence of `.env` files, `config.env`.

8.  **Key Module/Directory Identification**: 
    *   For Go: Identify `cmd/`, `internal/`, `pkg/`, `api/`, `web/` directories.
    *   Extract module name from `go.mod`.

**Output of Analysis:**

The `internal/analyzer` produces a structured `AnalysisResults` object (or similar) containing all the detected information. This object is then cached by `internal/cache` and used by `internal/llm` to construct informed prompts.

**Claude's Task:**

When asked to understand a project or generate configurations, Claude should leverage the output of this static analysis. If the analysis needs to be extended or refined, Claude should propose modifications to the `internal/analyzer` to detect new patterns or extract additional context relevant to AI-driven tasks.