# 🌱 ccb — Claude Code Bootstrapper

> Point it at any repo. It reads the code, asks what matters, and writes a tailored Claude Code config — CLAUDE.md, agents, rules, skills, hooks — in under a minute.

`ccb` is a macOS CLI that removes the boring "set up Claude Code for my project" step. Instead of copy-pasting a generic template, you get a config grounded in your actual code: real file paths, real modules, real conventions. It works on solo Go scripts and enterprise Laravel monorepos alike.

---

## Install

One line, no Go toolchain required:

```bash
curl -fsSL https://raw.githubusercontent.com/abdessama-cto/ccb/master/install.sh | bash
```

The installer drops a native Apple Silicon binary into `~/.local/bin/ccb` and makes sure it's on your `PATH`. If you've already got `gh`, `jq`, and an AI provider API key handy you're ready to go.

Need another arch or want to build from source?

```bash
git clone https://github.com/abdessama-cto/ccb.git
cd ccb
go build -o ~/.local/bin/ccb .
```

---

## Quick start

First time:

```bash
ccb start
```

That walks you through picking a language, AI provider (OpenAI / Gemini / Ollama), API key, model, and profile — then bootstraps the current directory.

Already configured? Just run:

```bash
ccb              # or: ccb init
```

Or on a GitHub URL (ccb clones into `~/.ccb/projects/<name>/` first):

```bash
ccb init https://github.com/owner/repo
```

---

## What you get

After a run, your project has:

```
CLAUDE.md                            # stack · key modules · conventions · guidance for Claude
.claude/
  settings.json                      # permissions, env, allow/ask/deny patterns
  rules/
    01-core-behavior.md              # how Claude should collaborate on this repo
    02-git-workflow.md               # branching, commits, PR flow
    03-testing.md                    # testing expectations for this stack
    04-code-quality.md               # quality + security pointers
    05-project-specific.md           # guidance unique to THIS project
  agents/
    <your-agents>.md                 # specialized sub-assistants (tailored)
  skills/
    <your-skills>.md                 # reusable procedures Claude can run
  commands/
    context.md · progress.md · review.md · ship.md · test.md
  hooks/
    pre-edit-secret-scan.sh          # blocks edits that would introduce secrets
    post-edit-format.sh              # auto-formats edited files
    …                                # others depending on profile
docs/
  architecture.md                    # module breakdown, data flow, integrations
  progress.md                        # rolling session log (append-only)
```

Every file is grounded in your code — CLAUDE.md cites real modules, agents reference real file paths, rules are guidance ("when X, here's how this project does Y") rather than commandments.

---

## Commands

| Command | What it does |
|---|---|
| `ccb start` | First-time guided setup then bootstrap. Recommended entry point. |
| `ccb init [repo-or-dir]` | Bootstrap the current dir, a GitHub URL, or `.` |
| `ccb settings` | Interactive wizard: provider, key, model, language, profile |
| `ccb add agent [name]` | Ask the AI to design a new agent tailored to the project |
| `ccb add skill [query]` | Search skills.sh and install a skill into `.claude/skills/` |
| `ccb add rule <text>` | Append a project-specific rule |
| `ccb list` | Show what's installed in the current project |
| `ccb sync` | Verify skills.sh is reachable |
| `ccb doctor` | Sanity-check your setup (bin, config, hooks, providers) |
| `ccb uninstall` | Remove everything ccb wrote — using the manifest, not a guess |
| `ccb update` | Self-update to the latest release |
| `ccb version` | Print version + host arch |

---

## How it works

```
  your repo
     │
     ▼
  [1] static analysis      ← detects stack, tests, CI, Docker, .env
     │
     ▼
  [2] AI understanding     ← LLM reads source; produces ProjectUnderstanding JSON
     │                       (batched automatically if the codebase is huge)
     ▼
  [3] AI wizard            ← LLM asks 1-10 questions specific to THIS project
     │
     ▼
  [4] AI proposals         ← LLM suggests agents, rules, skills (you pick)
     │
     ▼
  [5] AI generation        ← one delimited-block LLM call writes every .md
     │
     ▼
  [6] deterministic files  ← settings.json, hooks, slash commands (no LLM needed)
     │
     ▼
  [7] manifest + backup    ← .ccb/.ccb-manifest.json tracks every file written
```

The LLM only generates content that benefits from creativity (prose, guidance). Infrastructure files (settings.json, hooks, commands) are written deterministically so they're always correct, never hallucinated.

---

## Supported LLM providers

- **OpenAI** — GPT-5.x, GPT-4o, o1 family
- **Google Gemini** — Gemini 3 preview, 2.5 Pro/Flash, 2.0, 1.5 Pro
- **Ollama** — anything local (Llama 3, Qwen, DeepSeek Coder, etc.)

Pick any of them in `ccb settings`. You can switch at any time; `ccb init` will re-run with the new provider.

---

## Requirements

- macOS Apple Silicon or Intel (the installer auto-detects)
- `gh` — only if you want to bootstrap a GitHub URL directly (`brew install gh`)
- `jq` — used by the generated hooks (`brew install jq`)
- An API key for OpenAI or Gemini, **or** a running Ollama daemon on `http://localhost:11434`

Run `ccb doctor` any time to see what's missing.

---

## Configuration

| Path | Purpose |
|---|---|
| `~/.ccb/config.yaml` | Global config: provider, keys, model, language, profile |
| `~/.ccb/projects/` | Cloned GitHub repos ccb has worked on |
| `<project>/.ccb/analysis.json` | Cached semantic analysis (invalidate via `ccb reanalyze` — coming soon) |
| `<project>/.ccb/.ccb-manifest.json` | List of files ccb wrote — used by `ccb uninstall` |

Legacy `.ccbootstrap/` directories (from before v0.9.5) are auto-migrated on first `ccb init`.

---

## Contributing — 👋 you're welcome

ccb is early-stage and has real rough edges. If you run into one, please tell us — a one-line GitHub issue is genuinely helpful.

Good first contributions:
- **Share what ccb produced for your project** (good, bad, weird) — open an issue with a snippet. This is how we tune the prompts.
- **Add a stack detector** in `internal/analyzer/` for a language or framework ccb misses (Rust workspaces, Elixir/Phoenix, Kotlin Spring, etc.).
- **Improve a generator prompt** in `internal/llm/` — if you have a better way to phrase the agent or rules prompt, send a PR.
- **Add a hook or slash command template** in `internal/generator/generator.go` that would save developers time.
- **Report or fix LLM-parse edge cases** in `internal/llm/generate.go` — new parse failures are high-signal and reproducible from a saved raw response.

Dev loop:

```bash
git clone https://github.com/abdessama-cto/ccb.git
cd ccb
go build ./... && go test ./...
go build -o ~/.local/bin/ccb .   # install your local build
ccb start                         # test it
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide — expectations, style, how to propose a change.

---

## License

MIT — see [LICENSE](LICENSE).

---

## Credits

Built on top of [Claude Code](https://claude.com/claude-code), [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea), [spf13/cobra](https://github.com/spf13/cobra), and the [skills.sh](https://skills.sh) catalog.
