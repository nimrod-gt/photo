# /todo order

Hard rule: use **only** the injected `todo.json` (Read it only if injection failed). No Glob, Grep,
Bash or agents. Do not modify the file.

1. Split todos into `in_progress` and `open`.
2. Rank `open` todos with these criteria, in order of weight:
   1. dependencies: every id in `depends_on` must be absent from `todos[]`; todos with unsatisfied
      dependencies sink to the bottom (mention which ids block them);
   2. nature, judged from description and context: bug / crash / data loss > incorrect behaviour >
      UX annoyance > new feature > refactor / cleanup;
   3. size, judged from description, context and `docs` count: smaller first;
   4. age: older `created_at` first.
   Plan existence, step count and unanswered questions do **not** change the rank — `start` takes
   care of those.
3. Output:
   - at most 5 ranked lines: `1. #<id> — <description> — <why now, one clause>`;
   - then, if any todo is `in_progress`: `in progress: #a, #b`;
   - then, if any open todo has `plan == null`: `unplanned: #a, #b — run /todo plan`.
   If there are no open todos, print `nothing to start` (plus the `in progress:` line if any).
