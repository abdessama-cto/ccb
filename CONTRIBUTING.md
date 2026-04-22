# Contributing to ccb

Thanks for considering a contribution! ccb is small, practical, and welcomes pull requests of any size — from a typo fix to a new stack detector.

## Before you start

For anything bigger than a one-file tweak, please open an issue first or drop a line in a draft PR. It's much easier to steer a change early than to reshape a finished PR.

Good changes to propose without pre-discussion:
- Bug fixes with a clear reproducer
- New stack detectors in `internal/analyzer/`
- Better error messages, better UX copy, better spinner labels
- Test coverage for existing behavior
- README / docs improvements

Things worth a quick issue first:
- Prompt rewrites in `internal/llm/*.go`
- New top-level commands
- New hook templates (they ship to every user — we want to review intent)
- Anything that changes `~/.ccb/config.yaml` or `.claude/settings.json` shape

## Dev setup

```bash
git clone https://github.com/abdessama-cto/ccb.git
cd ccb
go build ./... && go test ./...
go build -o ~/.local/bin/ccb .
ccb start
```

You'll need:
- Go 1.21+
- `gh` and `jq` (the generated hooks use `jq`)
- An API key for at least one LLM provider (or Ollama running locally)

## Repo tour

| Path | What's there |
|---|---|
| `cmd/ccbootstrap/` | Every `ccb <command>` lives here (Cobra commands) |
| `internal/analyzer/` | Static analysis: stack detection, semantic context extraction, file ranking |
| `internal/llm/` | Everything that talks to an LLM. Prompt builders, response parsers, sanitizer |
| `internal/generator/` | Writes files to disk (deterministic ones) + manifest + hooks |
| `internal/tui/` | Shared TUI primitives (colors, spinner, Box) |
| `internal/config/` | Global config loading + migration |
| `internal/cache/` | Per-project analysis cache |
| `internal/skills/` | skills.sh client |
| `internal/github/` | `gh` wrapper |

## Coding style

- Gofmt on save (pre-commit handles it).
- Prefer small, composable functions. Aim for files under ~400 lines.
- Comments explain *why*, not *what* — unless the "what" is non-obvious.
- Error messages are a UX surface. Use `fmt.Errorf("context: %w", err)` with a concrete verb ("fetching skill …", "reading cache …").

## Tests

We don't require test coverage for every change, but anything touching `internal/llm/generate.go` parsing or `internal/llm/llm.go` sanitization **must** come with a regression test. Those paths have broken silently in the past and tests caught it.

Run:
```bash
go test ./...
```

## Commit style

Conventional Commits:
```
feat: short description
fix: short description
docs: short description
chore: short description
```

One logical change per commit. If your PR touches unrelated files, split into multiple commits so review can go module by module.

## Submitting a PR

1. Fork, create a branch off `master`: `git checkout -b feat/my-thing`
2. Make your change + run `go build ./... && go test ./...`
3. Push and open a PR with:
   - **What** changed and **why**
   - **How** to test (commands to run)
   - Screenshots/terminal output for TUI-facing changes

## Reporting bugs

When ccb does something weird on your project, the most useful issue contains:
- `ccb version` output
- The command you ran and the full output (redact any API keys)
- Your `~/.ccb/config.yaml` minus the keys (provider + model is enough)
- For parse failures: the raw LLM response if you can capture it

## Code of conduct

Be decent to each other. Assume good faith. Disagree on ideas, not on people. That's it.

## License

By contributing, you agree that your contributions will be licensed under the MIT License (see [LICENSE](LICENSE)).
