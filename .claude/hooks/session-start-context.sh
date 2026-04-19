#!/bin/bash
# .claude/hooks/session-start-context.sh — Inject git context (SessionStart)
BRANCH=$(git branch --show-current 2>/dev/null || echo "detached")
LAST_COMMIT=$(git log -1 --pretty="%s (%cr)" 2>/dev/null || echo "none")
UNCOMMITTED=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
PROGRESS=""
[[ -f docs/progress.md ]] && PROGRESS=$(tail -30 docs/progress.md)

cat <<EOF
{
  "additionalContext": "Git: $BRANCH | Last: $LAST_COMMIT | Uncommitted: $UNCOMMITTED files\n\nRecent progress:\n$PROGRESS"
}
EOF
