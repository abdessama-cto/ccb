#!/bin/bash
# .claude/hooks/pre-bash-audit.sh — Audit log for bash commands (PreToolUse)
CMD=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.command // ""' 2>/dev/null)
[[ -z "$CMD" ]] && exit 0

TIMESTAMP=$(date -Iseconds 2>/dev/null || date)
SESSION="${CLAUDE_SESSION_ID:-unknown}"
LOG_DIR=".claude/audit"
mkdir -p "$LOG_DIR"
printf '%s [session:%s] %s\n' "$TIMESTAMP" "$SESSION" "$CMD" >> "$LOG_DIR/bash-commands.log"
exit 0
