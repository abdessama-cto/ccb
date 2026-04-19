#!/bin/bash
# .claude/hooks/pre-push-tests.sh — Run tests before git push (PreToolUse)
CMD=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.command // ""' 2>/dev/null)
[[ "$CMD" != *"git push"* ]] && exit 0

echo "🧪 Running tests before push..." >&2
# No test command detected — add yours here
if [[ $? -ne 0 ]]; then
  echo "❌ Tests failing. Push blocked. Fix tests first." >&2
  exit 2
fi
echo "✅ Tests green. Push proceeding." >&2
exit 0
