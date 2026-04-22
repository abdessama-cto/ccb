<!--
Thanks for sending a PR! A few lines below help us review faster.
Delete any section that doesn't apply.
-->

## What

<!-- One or two sentences describing the change -->

## Why

<!-- The motivation: bug report, feature request, follow-up to a past PR, etc. -->

## How to test

<!-- The commands a reviewer should run locally. Example:
  go build ./... && go test ./...
  ccb start
  (then what to look for)
-->

## Screenshots / terminal output

<!-- Optional but very helpful for TUI-facing or generation-prompt changes -->

## Checklist

- [ ] `go build ./... && go vet ./... && go test ./...` passes
- [ ] New or changed behavior has a test if it lives in `internal/llm/` parsing or sanitization paths
- [ ] No secrets or API keys in the diff
- [ ] Commit messages follow Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`)
