# /todo finish <n>

Requires a todo id. Uses Read, `AskUserQuestion` and Write only — no git, no source code.

1. Print the todo: `#<id>`, description, status, branch (if set), `plan.summary` (if planned).
2. Ask with `AskUserQuestion`:
   - question: `Delete todo #<id> "<description>"? This removes it from todo.json; the git branch is not touched.`
   - options: `Delete` — remove the entry permanently; `Keep` — leave todo.json unchanged.
3. `Keep` → print `kept #<id>` and stop.
4. `Delete` → Read `todo.json`, remove the entry with that id (if it is already gone, print
   `todo #<id> not found` and stop), leave `next_id` as is, Write the whole file, print
   `deleted #<id>`. If `branch` was set, add one line: `branch <name> was not deleted`.
