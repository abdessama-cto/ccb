# Brief Application — `ccbootstrap` v3.0

**Claude Code Project Bootstrapper pour Mac Apple Silicon**
*Un outil CLI natif Mac M-series, installable via un one-liner shell, qui analyse n'importe quel repo GitHub et génère automatiquement une configuration Claude Code complète. Tout est automatisé : clone, analyse, questionnaire, génération, commit, push, PR. 15 minutes chrono.*

**Version 3.0 — Changes** :
- Distribution via script `.sh` (plus de npm)
- Natif Mac Apple Silicon (M1/M2/M3/M4 arm64-darwin)
- Hooks refactorisés : tous justifiés, zéro doublon avec permissions
- Process entièrement automatisé
- Agent conseiller GPT-5.4 en mode opt-in
- Menu settings complet

---

## 1. Installation — one-liner pour Mac

### Installation principale

```bash
curl -fsSL https://ccbootstrap.dev/install.sh | bash
```

### Ce que fait le script d'install

```bash
#!/usr/bin/env bash
set -euo pipefail

# 1. Vérifie Mac Apple Silicon
if [[ "$(uname)" != "Darwin" ]] || [[ "$(uname -m)" != "arm64" ]]; then
  echo "❌ ccbootstrap v3.0 requires Mac Apple Silicon (M1/M2/M3/M4)"
  echo "For Intel Mac or Linux, see https://ccbootstrap.dev/other-platforms"
  exit 1
fi

# 2. Vérifie/installe Homebrew
if ! command -v brew &>/dev/null; then
  echo "📦 Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# 3. Installe les dépendances via brew (idempotent)
echo "📦 Checking dependencies..."
for dep in git gh jq node; do
  if ! brew list "$dep" &>/dev/null; then
    echo "  Installing $dep..."
    brew install "$dep"
  fi
done

# 4. Télécharge le binaire ccbootstrap (arm64-darwin natif)
CCBOOTSTRAP_VERSION="${CCBOOTSTRAP_VERSION:-latest}"
BINARY_URL="https://github.com/ccbootstrap/cli/releases/${CCBOOTSTRAP_VERSION}/download/ccbootstrap-darwin-arm64"
BIN_DIR="${HOME}/.local/bin"
mkdir -p "$BIN_DIR"

echo "⬇️  Downloading ccbootstrap ${CCBOOTSTRAP_VERSION}..."
curl -fsSL -o "${BIN_DIR}/ccbootstrap" "$BINARY_URL"
chmod +x "${BIN_DIR}/ccbootstrap"

# 5. Ajoute au PATH si nécessaire
if [[ ":$PATH:" != *":${BIN_DIR}:"* ]]; then
  SHELL_RC="${HOME}/.zshrc"  # macOS default depuis Catalina
  [[ -f "${HOME}/.bashrc" ]] && SHELL_RC="${HOME}/.bashrc"
  echo "export PATH=\"${BIN_DIR}:\$PATH\"" >> "$SHELL_RC"
  echo "✅ Added to PATH in ${SHELL_RC} (restart shell or run: source ${SHELL_RC})"
fi

# 6. Crée le dossier de config
mkdir -p "${HOME}/.ccbootstrap"

# 7. Premier run pour setup
echo "🎉 Installation complete!"
echo ""
echo "Next step:"
echo "  ccbootstrap init https://github.com/<user>/<repo>"
echo ""
echo "Or configure settings first:"
echo "  ccbootstrap settings"
```

### Désinstallation

```bash
curl -fsSL https://ccbootstrap.dev/uninstall.sh | bash
```

### Update

```bash
ccbootstrap self-update
```

---

## 2. Stack technique

### Langage & build
- **Core** : Go 1.22+ (binaire unique, compilé `arm64-darwin`, zéro dépendance runtime)
- **Distribution** : GitHub Releases (binaires pré-compilés)
- **Install** : Shell script standalone

Pourquoi Go :
- Single binary, ~15 MB, startup instantané
- Excellent support arm64-darwin natif
- Cross-compilation si besoin (Intel, Linux) plus tard
- Performance terminal UI très rapide

### Dépendances runtime (gérées par install.sh)
- `git` (2.40+) — opérations git
- `gh` (GitHub CLI 2.50+) — auth GitHub, PR creation
- `jq` — JSON parsing dans les hooks générés
- `node` (20+ LTS) — uniquement pour lancer `npx skills` qui fait partie de la génération

### Outils internes utilisés
- **tview** + **tcell** — interface TUI riche (menus, prompts, bulles)
- **go-git** — opérations git inline sans shell-out
- **sashabaranov/go-openai** — client OpenAI GPT-5.4
- **go-github** — API GitHub
- **viper** — gestion config

### Où sont stockées les données
```
~/.ccbootstrap/
├── config.yaml              # Settings globaux (chiffrés pour les keys)
├── cache/
│   ├── templates/           # Templates téléchargés
│   └── skills-leaderboard.json  # Cache skills.sh (refresh 24h)
├── projects/                # Repos clonés
│   └── <owner>-<repo>/
├── logs/                    # Logs audit
└── usage.db                 # SQLite — tracking usage OpenAI
```

---

## 3. Process entièrement automatisé

### La commande unique

```bash
ccbootstrap init https://github.com/user/repo
```

### Ce qui se passe — 8 étapes automatiques

```
Étape 1 [auto] : Auth GitHub (utilise gh auth si déjà connecté)
Étape 2 [auto] : Clone du repo dans ~/.ccbootstrap/projects/
Étape 3 [auto] : Analyse statique du code (stack, LOC, tests, conventions)
Étape 4 [user] : Questionnaire interactif (10 questions, ? pour IA)
Étape 5 [auto] : Génération de toute la config (CLAUDE.md, .claude/, docs/)
Étape 6 [auto] : Installation des skills.sh recommandées (npx skills add)
Étape 7 [auto] : Vérification — lance les tests existants pour non-régression
Étape 8 [auto] : Git operations — branch + commit + push + PR via gh CLI
```

### Exemple de session chronométrée

```
$ ccbootstrap init https://github.com/afdal-ma/new-bank-integration

🌱 ccbootstrap v3.0.0 — Mac Apple Silicon (M3 Pro)

[00:00] ✓ GitHub auth OK (via gh CLI, user: @afdal-ma)
[00:02] ✓ Cloning repo... (3.1s, 847 commits, 32k LOC)
[00:05] ⠋ Analyzing codebase...
[00:22] ✓ Analysis complete

📊 Project fingerprint
   Stack      : Laravel 10 + Vue.js 2 + PostgreSQL 15
   Size       : 32,145 LOC across 412 files
   Age        : Brownfield (847 commits, 18 months)
   Tests      : PHPUnit, 73% coverage
   CI/CD      : GitHub Actions (.github/workflows/ci.yml)
   Conventions: PSR-12, ESLint, Prettier pre-configured
   Secrets    : .env present (excluded from git ✓)

📝 Quick setup — 10 questions (? for AI advice on any question)

[1/10] ? Primary goal for this project?
  > Quality (recommended for banking integrations)
    Ship fast
    Stability-first
    Refactor-focused

[2/10] ? Workflow style?
  > plan + execute (AI recommended for your profile) 
    vibe coding
    spec-driven strict

... (8 more questions)

[03:45] ✓ Questionnaire done

[03:46] ⠋ Generating configuration...
         Writing CLAUDE.md (127 lines)...
         Writing .claude/settings.json...
         Writing .claude/rules/ (4 files)...
         Writing .claude/agents/ (3 subagents)...
         Writing .claude/commands/ (5 commands)...
         Writing .claude/hooks/ (7 scripts)...
         Writing docs/ structure...
[03:58] ✓ Config generated

[03:59] ⠋ Installing skills from skills.sh...
         npx skills add vercel-labs/skills --skill find-skills
         npx skills add anthropics/skills
         npx skills add better-auth/skills
[04:52] ✓ 3 skills installed

[04:53] ⠋ Running existing test suite for regression check...
         php artisan test --parallel
[05:31] ✓ All tests green (247 passed, 0 failed)

[05:32] ⠋ Git operations...
         Creating branch: ccbootstrap/initial-setup
         Staging 23 new files
         Commit: "chore(claude): bootstrap Claude Code via ccbootstrap v3.0"
         Pushing to origin...
         Creating PR via gh CLI...
[05:48] ✓ PR #42 created

🎉 Done in 5 minutes 48 seconds

📎 PR: https://github.com/afdal-ma/new-bank-integration/pull/42

Next steps:
  cd ~/.ccbootstrap/projects/afdal-ma-new-bank-integration
  claude
  Then run: /context to verify token usage
```

### Flags pour contrôler l'automatisation

```bash
# Mode totalement non-interactif (CI/CD)
ccbootstrap init <repo> --yes --profile balanced

# Dry-run (génère sans pusher)
ccbootstrap init <repo> --dry-run

# Skip questionnaire (defaults intelligents)
ccbootstrap init <repo> --skip-questionnaire

# Custom branch name
ccbootstrap init <repo> --branch "feat/claude-setup"

# No PR, just push
ccbootstrap init <repo> --no-pr

# Profil prédéfini
ccbootstrap init <repo> --profile strict     # spec-driven + tous les hooks
ccbootstrap init <repo> --profile balanced   # plan+execute + hooks essentiels
ccbootstrap init <repo> --profile lightweight # vibe-friendly + minimal
```

---

## 4. Hooks — VRAIMENT justifiés (zéro doublon avec permissions)

### Principe de sélection
Un hook est inclus dans la génération **si et seulement si** il résout un problème que les permissions `allow/deny/ask` **ne peuvent pas résoudre**.

Les permissions font du **filtrage statique binaire par pattern**. Les hooks font tout le reste.

### Les 7 hooks générés — tous justifiables

#### Hook 1. Auto-format après édition (`PostToolUse`)
**Ce que ça fait** : format automatiquement tout fichier écrit/modifié avec l'outil approprié (prettier, black, ruff, gofmt, rubocop...).

**Pourquoi pas permission** : les permissions ne peuvent **pas déclencher d'action**. Elles filtrent, c'est tout.

```bash
#!/bin/bash
# .claude/hooks/post-edit-format.sh
FILE=$(jq -r '.tool_input.file_path // .tool_input.path // ""')
[[ -z "$FILE" ]] && exit 0

case "$FILE" in
  *.php) command -v ./vendor/bin/pint && ./vendor/bin/pint "$FILE" ;;
  *.js|*.ts|*.vue|*.jsx|*.tsx) npx prettier --write "$FILE" ;;
  *.py) ruff format "$FILE" ;;
  *.go) gofmt -w "$FILE" ;;
  *.rb) rubocop -a "$FILE" ;;
esac
exit 0
```

#### Hook 2. Injection de contexte git au démarrage (`SessionStart`)
**Ce que ça fait** : injecte la branche actuelle, le dernier commit, le nombre de fichiers non-commités, et le contenu de `docs/progress.md` dans le contexte de Claude.

**Pourquoi pas permission** : les permissions ne peuvent **pas injecter de contexte dynamique**, elles ne font que filtrer les outils.

```bash
#!/bin/bash
# .claude/hooks/session-start-context.sh
BRANCH=$(git branch --show-current 2>/dev/null || echo "detached")
LAST_COMMIT=$(git log -1 --pretty="%s (%cr)" 2>/dev/null || echo "none")
UNCOMMITTED=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
PROGRESS=""
[[ -f docs/progress.md ]] && PROGRESS=$(head -50 docs/progress.md)

cat <<EOF
{
  "additionalContext": "Git: $BRANCH | Last: $LAST_COMMIT | Uncommitted: $UNCOMMITTED\n\nCurrent progress:\n$PROGRESS"
}
EOF
```

#### Hook 3. Scan de secrets dans le contenu écrit (`PreToolUse`)
**Ce que ça fait** : détecte si le contenu à écrire contient des patterns de secrets (API keys, tokens, passwords hardcodés) — peu importe le nom du fichier.

**Pourquoi pas permission** : les permissions filtrent par **nom de fichier** uniquement, pas par **contenu**. Un `.env.example` légitime passe au travers de `deny: "Write(.env*)"`, mais un secret hardcodé dans `config/app.php` passe aussi. Ce hook check le contenu effectif.

```bash
#!/bin/bash
# .claude/hooks/pre-edit-secret-scan.sh
CONTENT=$(jq -r '.tool_input.content // .tool_input.new_string // ""')
[[ -z "$CONTENT" ]] && exit 0

# Patterns de secrets connus
PATTERNS=(
  'AKIA[0-9A-Z]{16}'                        # AWS Access Key
  'sk-[a-zA-Z0-9]{48}'                      # OpenAI key
  'sk-ant-[a-zA-Z0-9-]{95}'                 # Anthropic key
  'ghp_[a-zA-Z0-9]{36}'                     # GitHub PAT
  'AIza[0-9A-Za-z\\-_]{35}'                 # Google API key
  'xox[baprs]-[0-9a-zA-Z-]{10,48}'          # Slack token
  'BEGIN (RSA |DSA |EC |OPENSSH |)PRIVATE KEY'  # Private keys
)

for pattern in "${PATTERNS[@]}"; do
  if echo "$CONTENT" | grep -qE "$pattern"; then
    echo "🚫 Secret pattern detected: $pattern" >&2
    echo "Remove the secret or use environment variables instead." >&2
    exit 2
  fi
done
exit 0
```

#### Hook 4. Auto-commit après édition successful (`PostToolUse`)
**Ce que ça fait** : stage le fichier modifié et crée un commit atomique avec message auto-généré. Permet d'avoir un historique granulaire plutôt qu'un mega-commit en fin de session.

**Pourquoi pas permission** : action post-événement — impossible avec des permissions.

```bash
#!/bin/bash
# .claude/hooks/post-edit-auto-commit.sh
FILE=$(jq -r '.tool_input.file_path // .tool_input.path // ""')
[[ -z "$FILE" ]] && exit 0
[[ ! -d .git ]] && exit 0

# Skip si on est sur main/master (pas d'auto-commit direct sur prod)
BRANCH=$(git branch --show-current)
[[ "$BRANCH" == "main" || "$BRANCH" == "master" ]] && exit 0

# Stage + commit atomique
git add "$FILE" 2>/dev/null || exit 0
SHORT_PATH=$(basename "$FILE")
git commit -m "chore(ai): edit $SHORT_PATH" --no-verify 2>/dev/null || true
exit 0
```

#### Hook 5. Notification desktop sur Stop (`Stop`)
**Ce que ça fait** : envoie une notification macOS quand Claude termine une tâche longue, permet de switcher vers d'autres tâches sans fixer l'écran.

**Pourquoi pas permission** : side effect externe (interaction avec macOS Notification Center) — hors scope des permissions.

```bash
#!/bin/bash
# .claude/hooks/stop-notify.sh
osascript -e 'display notification "Claude Code has finished your task" with title "ccbootstrap" sound name "Glass"'
exit 0
```

#### Hook 6. Tests obligatoires avant `git push` (`PreToolUse`)
**Ce que ça fait** : intercepte `git push` et lance la suite de tests. Bloque si échec, laisse passer si succès.

**Pourquoi pas permission** : les permissions ne peuvent **pas exécuter de logique** — elles matchent un pattern et répondent allow/deny, elles ne peuvent pas lancer `php artisan test` et décider selon le résultat.

```bash
#!/bin/bash
# .claude/hooks/pre-push-tests.sh
CMD=$(jq -r '.tool_input.command // ""')
[[ "$CMD" != *"git push"* ]] && exit 0

echo "🧪 Running tests before push..." >&2

# Détection stack
if [[ -f artisan ]]; then
  php artisan test --parallel
elif [[ -f package.json ]]; then
  npm test
elif [[ -f pyproject.toml ]]; then
  pytest
elif [[ -f go.mod ]]; then
  go test ./...
else
  exit 0  # Pas de stack détectée, on laisse passer
fi

if [[ $? -ne 0 ]]; then
  echo "❌ Tests failing. Push blocked. Fix tests first." >&2
  exit 2
fi
echo "✅ Tests green, push proceeding" >&2
exit 0
```

#### Hook 7. Audit log des commandes Bash (`PreToolUse`)
**Ce que ça fait** : loge chaque commande Bash exécutée par Claude avec timestamp, dans un fichier d'audit.

**Pourquoi pas permission** : side effect (écriture dans fichier) — pas un filtre.

**Pourquoi c'est utile** : traçabilité pour audits de sécurité, debugging de sessions longues, compliance (SOC2, ISO 27001).

```bash
#!/bin/bash
# .claude/hooks/pre-bash-audit.sh
CMD=$(jq -r '.tool_input.command // ""')
TIMESTAMP=$(date -Iseconds)
SESSION="${CLAUDE_SESSION_ID:-unknown}"
LOG_DIR=".claude/audit"
mkdir -p "$LOG_DIR"
printf '%s [session:%s] %s\n' "$TIMESTAMP" "$SESSION" "$CMD" >> "$LOG_DIR/bash-commands.log"
exit 0
```

### Les hooks que j'ai SUPPRIMÉS du brief v1

| Hook supprimé | Pourquoi |
|---|---|
| Bloquer `rm -rf` | Faisable par `deny: "Bash(rm -rf*)"` en permission |
| Bloquer édition `.env` | Faisable par `deny: "Write(.env)"` en permission |

Ces deux cas sont des filtres statiques — mieux gérés par les permissions, plus rapides, pas de shell-out.

---

## 5. Permissions générées (complément des hooks)

```json
{
  "permissions": {
    "allow": [
      "Bash(npm test*)",
      "Bash(php artisan test*)",
      "Bash(pytest*)",
      "Bash(go test*)",
      "Bash(git status)",
      "Bash(git diff*)",
      "Bash(git log*)",
      "Bash(ls*)",
      "Bash(cat*)",
      "Bash(grep*)",
      "Read(*)"
    ],
    "deny": [
      "Bash(rm -rf*)",
      "Bash(rm -fr*)",
      "Bash(sudo rm*)",
      "Bash(git push --force*)",
      "Bash(git reset --hard*)",
      "Write(.env)",
      "Write(.env.local)",
      "Write(.env.production)",
      "Write(**/secrets/**)",
      "Read(**/*.pem)",
      "Read(**/*.key)",
      "Read(**/id_rsa)"
    ],
    "ask": [
      "Bash(git push*)",
      "Bash(git reset*)",
      "Bash(composer update)",
      "Bash(npm install)",
      "Bash(php artisan migrate:*)"
    ]
  }
}
```

### Distinction nette — ce que gère quoi

| Problème | Géré par |
|---|---|
| Bloquer suppression récursive | **Permission** `deny` |
| Bloquer écriture `.env` | **Permission** `deny` |
| Protéger clés privées en lecture | **Permission** `deny` |
| Demander confirmation avant `git push` | **Permission** `ask` |
| Auto-format après édition | **Hook** PostToolUse |
| Scan contenu pour secrets (tout fichier) | **Hook** PreToolUse |
| Tests avant `git push` | **Hook** PreToolUse avec logique |
| Notification desktop Stop | **Hook** Stop |
| Auto-commit | **Hook** PostToolUse |
| Injection contexte git | **Hook** SessionStart |
| Audit log | **Hook** PreToolUse |

**Zéro chevauchement.** Chaque mécanisme fait ce qu'il sait faire de mieux.

---

## 6. Agent conseiller GPT-5.4 (opt-in)

### Activation
Sur chaque question du questionnaire, tape `?` pour activer la bulle conseil.

### Modèle utilisé
- **Par défaut** : `gpt-5.4-nano` (rapide, ~200ms, bon marché)
- **Changeable** via `ccbootstrap settings` : `gpt-5.4-mini`, `gpt-5.4`, `gpt-5.4-thinking`

### Context envoyé au modèle
```json
{
  "model": "gpt-5.4-nano",
  "messages": [
    {"role": "system", "content": "You are a senior DevOps engineer advising on Claude Code setup. Be concise, specific, actionable. Never generic advice."},
    {"role": "user", "content": "Project context: {stack, LOC, team_size, tests_coverage, commits, age}\n\nPrevious answers: {...}\n\nCurrent question: {question}\nOptions: {options}\n\nWhat should I pick and why?"}
  ]
}
```

### Coût typique pour une session
- 10 questions × 500 tokens moyens = 5000 tokens
- Si l'utilisateur active `?` sur 3 questions : 15 000 tokens input + output
- GPT-5.4 nano : ~$0.005 par session complète

### Fallback
Si API OpenAI down ou clé invalide → dégradation gracieuse, le questionnaire continue sans la bulle, l'utilisateur voit un message info : `[?] unavailable — check ccbootstrap settings`.

---

## 7. Menu settings

```bash
ccbootstrap settings
```

```
⚙️  ccbootstrap Settings — v3.0.0
─────────────────────────────────

  🔑 Credentials
     [1] OpenAI API Key     sk-proj-••••••1234 ✅
     [2] GitHub Token       gho_••••••••5678 ✅ (via gh CLI)
     [3] Anthropic API Key  not set

  🤖 AI Assistant
     [4] Enabled            yes
     [5] Model              gpt-5.4-nano
     [6] Max tokens/session 20000 ($0.01 max)

  🎨 UI
     [7] Language           French
     [8] Color scheme       auto (follows system)
     [9] Verbosity          normal

  📦 Templates & Cache
     [10] Templates source   official (github.com/ccbootstrap/templates)
     [11] Auto-update        weekly
     [12] Cache location     ~/.ccbootstrap/cache
     [13] Clear cache

  🔧 Advanced
     [14] Auto-run tests after generation  yes
     [15] Create PR by default             yes
     [16] Default profile                  balanced
     [17] Skip confirmation prompts        no

  [u] Usage stats   [r] Reset all   [q] Quit   [s] Save

Choice: _
```

### Sous-menu configuration OpenAI

```
🔑 OpenAI API Configuration
───────────────────────────

  Current key : sk-proj-abc123...••••1234
  Status      : ✅ Valid (verified 2026-04-19 14:23 UTC)
  Model       : gpt-5.4-nano
  
  Usage this month:
    Requests   : 127
    Tokens     : 48,392
    Cost       : $0.12
    Budget     : $5.00 (2.4% used)

Actions:
  [1] Set/change API key
  [2] Test current key
  [3] Change default model
        • gpt-5.4-nano     (recommended — fast, cheap) ✅
        • gpt-5.4-mini     (balanced)
        • gpt-5.4          (powerful, slower)
        • gpt-5.4-thinking (deep reasoning, expensive)
  [4] Set monthly budget
  [5] View detailed usage
  [6] Enable/disable AI assistant entirely
  [7] Delete stored key

  [b] Back

Choice: _
```

### Storage des secrets
Les clés API sont **chiffrées** dans `~/.ccbootstrap/config.yaml` via macOS Keychain Services (natif, aucune bibliothèque tierce).

---

## 8. Intégration skills.sh — corrigée

**Pas d'API REST**. skills.sh est un **CLI wrapper** autour de packages GitHub.

### Commandes utilisées par ccbootstrap

```bash
# Recherche interactive
npx skills find [query]

# Installation
npx skills add <owner/repo>
npx skills add <owner/repo> --skill <skill-name>
npx skills add <owner/repo>@<version>

# Management
npx skills list       # liste installés
npx skills check      # check updates
npx skills update     # update all
```

### Comment ccbootstrap intègre ça

1. **Pendant l'analyse du repo** : détection de la stack
2. **Dans le questionnaire** : propose une liste curée basée sur :
   - La stack détectée (React → vercel-react-best-practices, Laravel → tailwind-design-system, etc.)
   - La popularité sur le leaderboard skills.sh (install count)
   - Cache local de `~/.ccbootstrap/cache/skills-leaderboard.json` (refresh 24h via scraping du site)
3. **À la génération** : exécute `npx skills add` pour chaque skill cochée par l'utilisateur

### Liste curée de skills recommandées par stack

```yaml
# templates/skills-by-stack.yaml
laravel:
  always_recommend:
    - vercel-labs/skills              # find-skills, le méta-skill
    - anthropics/skills               # Officiel (pdf, docx, pptx, frontend-design)
  conditional:
    - if_has_vue: hyf0/vue-skills
    - if_has_auth: better-auth/skills
    - if_has_api: vercel-labs/agent-skills  # composition patterns

nextjs:
  always_recommend:
    - vercel-labs/skills
    - vercel-labs/agent-skills        # vercel-react-best-practices
    - anthropics/skills
  conditional:
    - if_has_shadcn: shadcn/ui
    - if_has_auth: better-auth/skills

django:
  always_recommend:
    - vercel-labs/skills
    - anthropics/skills
  conditional:
    - if_has_api: obra/superpowers    # dispatching-parallel-agents, systematic-debugging

# ... etc pour chaque stack
```

### Pas de connexion API — tout via subprocess

```go
// pseudo-code Go
func InstallSkill(owner, repo, skillName string) error {
    args := []string{"skills", "add", owner + "/" + repo}
    if skillName != "" {
        args = append(args, "--skill", skillName)
    }
    cmd := exec.Command("npx", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

---

## 9. Le CLAUDE.md généré — exemple Laravel

```markdown
# Projet : new-bank-integration

Banking integration platform — multi-tenant Laravel service exposing
mortgage simulation APIs to partner banks.

## Stack
- Laravel 10.x, PHP 8.2+
- Vue.js 2 (frontend in `resources/js/`)
- PostgreSQL 15
- Redis (cache + queues)
- Docker Compose (local dev)

## Architecture
- Multi-tenant via `BelongsToTenant` trait on all models
- Tenant isolation: `tenant_id` discriminator column
- Auth: OAuth2 client_credentials for bank-to-bank API
- Async jobs via Horizon

## Commands
| Command | Purpose |
| --- | --- |
| `composer install && npm install` | Install dependencies |
| `php artisan migrate` | Run migrations |
| `php artisan test --parallel` | Run full test suite |
| `npm run dev` | Frontend watch mode |
| `./scripts/deploy.sh <env>` | Deploy to env |

## Conventions
- Controllers extend `TenantAwareController`
- Models declare `$with` for default eager loading
- Migrations always backward-compatible (no DROP COLUMN in prod)
- 1 feature test per API endpoint
- Commits follow Conventional Commits (feat:, fix:, chore:)

## Known pitfalls
- Never use `Model::all()` without tenant scope
- Async jobs must re-check tenant context
- Avoid `php artisan tinker` in production (multi-tenant not handled)

## Strict rules
- Never refactor code outside the scope of the requested task
- Always run `php artisan test` after model modifications
- Before proposing a solution, check `docs/solutions/` for similar problems

## References
- @docs/architecture.md — full architecture details
- @docs/decisions/ — Architecture Decision Records
- @docs/solutions/ — Solved problems with context
```

---

## 10. Spec release & distribution

### Release pipeline (GitHub Actions)
```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags: ['v*']

jobs:
  build-darwin-arm64:
    runs-on: macos-14  # Apple Silicon runner
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: |
          GOOS=darwin GOARCH=arm64 go build \
            -ldflags "-X main.version=${{ github.ref_name }}" \
            -o ccbootstrap-darwin-arm64 \
            ./cmd/ccbootstrap
      - uses: softprops/action-gh-release@v2
        with:
          files: ccbootstrap-darwin-arm64
```

### Distribution hosts
- **GitHub Releases** : binaires versionnés, source canonique
- **ccbootstrap.dev** : redirect vers la dernière release + `install.sh` hostée
- Future : **Homebrew tap** — `brew tap ccbootstrap/cli && brew install ccbootstrap`

### Signature & sécurité
- Tous les binaires **signés** avec certificat Apple Developer (notarization requise sur macOS 13+)
- `install.sh` vérifie le hash SHA256 avant installation
- Source ouverte sur GitHub pour audit

---

## 11. Ce qui n'est PAS dans v1.0 (hors scope)

- ❌ Pas de GUI (CLI uniquement)
- ❌ Pas d'Intel Mac / Linux / Windows (roadmap v1.5)
- ❌ Pas de GitLab/Bitbucket (GitHub only, roadmap v2.0)
- ❌ Pas d'agents teams auto-configurés (feature Claude Code expérimentale)
- ❌ Pas de mode multi-repo / monorepo avancé (v2.0)
- ❌ Pas de cloud sync des settings (v1.2)
- ❌ Pas de provider LLM autre qu'OpenAI (v1.3 : Anthropic, Gemini, Mistral)

---

## 12. Roadmap

| Version | Features | Date cible |
|---|---|---|
| v1.0 | Base fonctionnelle, Mac Apple Silicon, Go | +8 semaines |
| v1.1 | Templates additionnels (Rails, Spring, Go, Rust) | +12 semaines |
| v1.2 | Cloud sync des settings via iCloud | +16 semaines |
| v1.3 | Multi-provider LLM (Anthropic, Gemini) | +20 semaines |
| v1.5 | Intel Mac + Linux (binaries cross-compiled) | +24 semaines |
| v2.0 | GitLab, Bitbucket, monorepo, Homebrew tap officielle | +36 semaines |

---

## 13. Métriques de succès v1.0

| Métrique | Objectif à 3 mois |
|---|---|
| Temps d'installation (à partir du curl) | < 2 minutes |
| Temps de setup moyen (init → PR) | < 10 minutes |
| Taux de complétion du questionnaire | > 85% |
| Taux d'utilisation du bouton `?` (AI advice) | > 30% |
| Stacks supportées nativement | 8 minimum |
| Skills auto-installées par projet (moyenne) | > 3 |
| Subagents cochés par projet (moyenne) | > 2 |
| Downloads GitHub Releases premier mois | 1000+ |
| Stars GitHub premier mois | 300+ |

---

## 14. Risques & mitigations

| Risque | Probabilité | Mitigation |
|---|---|---|
| Changement breaking dans `npx skills` | Moyenne | Version pinning + tests auto hebdo |
| Release Claude Code casse le format config | Haute | Subscribe changelog, tests contre `claude --version` |
| macOS Gatekeeper bloque le binaire | Haute | Notarization Apple Developer dès la v0.1 |
| Rate limits GitHub API sur gros repos | Faible | Shallow clone + cache local |
| OpenAI API down pendant questionnaire | Faible | Fallback gracieux, questionnaire continue |
| Utilisateurs veulent Intel Mac | Moyenne | Cross-compile depuis M4 dès v1.1 |

---

## 15. Récapitulatif des points STRICTEMENT intégrés

Ce brief v3.0 prend en compte **tous** les points soulevés dans la discussion :

✅ **Structure `docs/`** avec `solutions/`, `brainstorms/`, `decisions/`
✅ **skills.sh** intégré comme wrapper CLI (`npx skills add`), **pas d'API REST**
✅ **Subagents best practices** : un rôle par agent, tools restrictifs, modèles hétérogènes (Opus pour critique, Sonnet par défaut, Haiku pour simple)
✅ **Top commandes** : les commands générées dans `.claude/commands/` couvrent les workflows les plus utilisés
✅ **Hooks pratiques et NON-REDONDANTS** : les 7 hooks sont tous justifiés, impossibles à faire via permissions seules
✅ **Optimisation contexte** : `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE=75`, `docs/progress.md`, hook `SessionStart` injection
✅ **Ordre de mise en place** : le CLI suit l'ordre recommandé (fondations → rules → hooks → skills → subagents)
✅ **Mode Docker** : supprimé en v1.0 car `.sh` natif Mac est plus simple et plus rapide — Docker remis dans la roadmap v1.5
✅ **Agent chat OpenAI GPT-5.4** : opt-in via `?` sur chaque question, context-aware, gpt-5.4-nano par défaut
✅ **Menu settings API** : configuration complète des clés, modèles, budgets, stockage Keychain macOS
✅ **Script `.sh`** : installation via `curl | bash`, natif Mac Apple Silicon, zéro dépendance npm obligatoire
✅ **Process tout-automatisé** : 8 étapes, dont 7 automatiques et 1 seule interactive (le questionnaire)

---

*Brief v3.0 — 2026-04-19 — Natif Mac Apple Silicon (M1/M2/M3/M4), distribution via shell script, process tout-automatisé.*
*Licence : CC-BY-4.0 pour le brief, MIT pour l'implémentation future.*
