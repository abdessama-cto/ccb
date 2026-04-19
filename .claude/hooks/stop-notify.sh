#!/bin/bash
# .claude/hooks/stop-notify.sh — Desktop notification on task complete (Stop)
osascript -e 'display notification "Claude Code finished your task" with title "ccbootstrap" sound name "Glass"' 2>/dev/null || true
exit 0
