---
description: Continue work on current branch by analyzing changes vs main
---

You are helping the user continue work on a feature branch by analyzing what has been done.

# Instructions

1. **Check current branch:**
   - Run: `git branch --show-current`
   - If on `main` or `master`, inform the user there's nothing to continue and suggest creating a feature branch

2. **Get branch history:**
   - Run: `git log main..HEAD --oneline` to see all commits in this branch
   - Run: `git log main..HEAD --format="%h %s" | wc -l` to count commits

3. **Get changed files:**
   - Run: `git diff main...HEAD --stat` to see summary of all changed files
   - Run: `git diff main...HEAD --name-only` to get list of modified files

4. **Check for uncommitted work:**
   - Run: `git status --short` to see any uncommitted changes
   - If there are uncommitted changes, note them separately

5. **Analyze the changes:**
   - Read the key modified files to understand what work was done
   - Focus on the main logic files (not tests, unless tests are the main work)
   - Identify the purpose and scope of the changes

# Report Format

Provide a summary in this structure:

## Branch Status
- Branch name: `<branch-name>`
- Commits ahead of main: X

## Work Completed
Brief summary of what has been implemented/changed:
- List main changes and their purpose
- Note any significant refactoring

## Files Changed
- List key modified files with brief description of changes
- Group by area if applicable

## Uncommitted Changes
- List any staged or unstaged changes that haven't been committed yet

## Suggested Next Steps
Based on the analysis:
- What appears to be incomplete or in progress
- Any TODOs or FIXMEs found in the changed code
- Potential issues that need attention

# Important Notes

- Focus on understanding the PURPOSE of the changes, not just listing them
- Keep the summary concise but informative
- Highlight any work that seems incomplete or needs attention
