---
name: interact-skills-sh-api
description: This skill provides Claude with the knowledge to interact programmatically with the `skills.sh` API. Claude will learn how to construct search queries, make HTTP GET requests to `https://skills.sh/api/search` and `https://raw.githubusercontent.com/.../SKILL.md`, parse the JSON responses, and extract relevant skill definitions for `ccb`'s skill discovery and installation features.
---

## Interacting with the `skills.sh` API

The `ccb` tool leverages `skills.sh` to discover and fetch Claude Code skills. The `internal/skills` package encapsulates this interaction. Claude needs to understand the API endpoints and expected data structures.

**1. Searching for Skills:**

*   **Endpoint**: `GET https://skills.sh/api/search?q=<query>`
*   **Purpose**: To find skills based on keywords (e.g., `go`, `docker`, `testing`).
*   **Request**: A simple HTTP GET request with a `q` query parameter.
*   **Expected Response (JSON Array)**:
    ```json
    [
      {
        "name": "go-project-structure",
        "description": "Guides on standard Go project layout and best practices.",
        "repo": "owner/repo",
        "path": "skills/go-project-structure",
        "branch": "main"
      },
      // ... more skill objects
    ]
    ```
*   **Claude's Task**: When a user asks to find skills, Claude should formulate a relevant `q` parameter, make the GET request, and parse this JSON array to present a list of matching skills.

**2. Fetching Raw Skill Content:**

*   **Endpoint**: `GET https://raw.githubusercontent.com/<owner>/<repo>/<branch>/<path>/SKILL.md`
*   **Purpose**: To retrieve the actual content of a `SKILL.md` file from its GitHub repository.
*   **Request**: Construct the URL using the `repo`, `branch`, and `path` fields obtained from the search results.
*   **Expected Response (Plain Text)**: The raw Markdown content of the `SKILL.md` file.
*   **Claude's Task**: Once a skill is selected, Claude should construct this URL and fetch the raw content. This content will then be used to create the local `.claude/skills/<skill-name>.md` file.

**Error Handling:**

*   Be prepared for HTTP errors (e.g., 404 Not Found, 500 Internal Server Error) from both `skills.sh` and `raw.githubusercontent.com`.
*   Handle cases where the JSON response from `skills.sh/api/search` is malformed or empty.
*   The `internal/skills` module will typically handle network requests and basic error propagation. Claude should focus on interpreting the successful responses and formulating correct requests.