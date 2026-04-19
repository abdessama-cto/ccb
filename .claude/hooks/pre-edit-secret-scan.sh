#!/bin/bash
# .claude/hooks/pre-edit-secret-scan.sh — Scan content for secrets (PreToolUse)
CONTENT=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.content // .new_string // ""' 2>/dev/null)
[[ -z "$CONTENT" ]] && exit 0

PATTERNS=(
  'AKIA[0-9A-Z]{16}'
  'sk-[a-zA-Z0-9]{48}'
  'sk-ant-[a-zA-Z0-9-]{95}'
  'ghp_[a-zA-Z0-9]{36}'
  'AIza[0-9A-Za-z\-_]{35}'
  'xox[baprs]-[0-9a-zA-Z-]{10,48}'
  'BEGIN (RSA |DSA |EC |OPENSSH |)PRIVATE KEY'
)

for pattern in "${PATTERNS[@]}"; do
  if echo "$CONTENT" | grep -qE "$pattern" 2>/dev/null; then
    echo "🚫 Secret pattern detected: $pattern" >&2
    echo "Remove the secret and use environment variables instead." >&2
    exit 2
  fi
done
exit 0
