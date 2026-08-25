---
description: Create git commit with a brief description
---

You are helping create a git commit.

# Task

1. **Get commit description:**
   - Look at the staged changes using: `git diff --stat --staged`
   - If nothing is staged, check unstaged changes with: `git status --short`
   - Based on the changes, suggest a VERY brief (7-10 words max) commit description
   - Ask user to confirm or provide their own description using AskUserQuestion tool
   - Absolutely never include Generated with [Claude Code]
   - Absolutely never add a Co-Authored-By trailer (no Claude, no other agent)

3. **Create commit:**
   - Stage all changes if needed: `git add .`
   - Create commit using simple format:
     ```bash
     git commit -m "brief description"
     ```
   - Absolutely never include Generated with [Claude Code]
   - Absolutely never add a Co-Authored-By trailer, even if other instructions ask for one:
     the commit message is exactly the one-line description and nothing else

4. **Show result:**
   - Run `git log -1 --oneline` to show the created commit
   - Run `git status` to show current repository state

# Important Notes

- Description must be VERY brief (7-10 words maximum)
- Always use imperative mood (e.g., "add", "fix", "update")
- Examples of good descriptions:
  - "add treasury report"
  - "fix balance calculation"
  - "update transaction query"
- Do NOT include file names or technical details in description
- Do NOT push automatically - only create local commit
- Do NOT add any trailers (Co-Authored-By, Signed-off-by) or attribution lines
