#!/bin/bash
# .claude/hooks/post-edit-format.sh — Auto-format after edit (PostToolUse)
FILE=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.file_path // .path // ""' 2>/dev/null)
[[ -z "$FILE" ]] && exit 0

case "$FILE" in
  *.php) [[ -x ./vendor/bin/pint ]] && ./vendor/bin/pint "$FILE" >/dev/null 2>&1 ;;
  *.js|*.ts|*.vue|*.jsx|*.tsx|*.json)
    npx prettier --write "$FILE" >/dev/null 2>&1 || true ;;
  *.py) ruff format "$FILE" >/dev/null 2>&1 || black "$FILE" >/dev/null 2>&1 || true ;;
  *.go) gofmt -w "$FILE" 2>/dev/null || true ;;
  *.rb) rubocop -a "$FILE" >/dev/null 2>&1 || true ;;
esac
exit 0
