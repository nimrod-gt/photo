#!/bin/bash
# Blocks:
# - git push (always)
# - git commit in main/master branches

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# 1. Block git push
if echo "$COMMAND" | grep -qE '^git\s+push(\s|$)'; then
    jq -n '{
      "hookSpecificOutput": {
        "hookEventName": "PreToolUse",
        "permissionDecision": "deny",
        "permissionDecisionReason": "git push is blocked. Push manually after reviewing changes."
      }
    }'
    exit 0
fi

# 2. Block git commit in main/master
if echo "$COMMAND" | grep -qE '^git\s+commit(\s|$)'; then
    BRANCH=$(git branch --show-current 2>/dev/null)
    if [[ "$BRANCH" == "main" || "$BRANCH" == "master" ]]; then
        jq -n --arg branch "$BRANCH" '{
          "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": ("git commit to " + $branch + " branch is blocked. Create a feature branch first.")
          }
        }'
        exit 0
    fi
fi

exit 0
