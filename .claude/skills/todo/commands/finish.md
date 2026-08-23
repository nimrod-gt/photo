# /todo finish <n>

Requires a todo id. Uses only the injected `todo.json`, `AskUserQuestion` and a Write.

1. Print the todo: `#<id>`, description, status, branch (if set), `plan.summary` (if planned).
2. Ask with `AskUserQuestion`:
   - question: `Delete todo #<id> "<description>"? This removes it from todo.json; the git branch is not touched.`
   - options: `Delete` — remove the entry permanently; `Keep` — leave todo.json unchanged.
3. `Keep` → print `kept #<id>` and stop.
4. `Delete` → re-read `todo.json`, remove the entry with that id, leave `next_id` as is, Write the
   whole file, print `deleted #<id>`. If `branch` was set, add one line: `branch <name> still exists`.
   Do not run any git command.
