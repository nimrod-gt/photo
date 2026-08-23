# /todo order

Hard rule: use **only** the injected `todo.json`. No Read, Glob, Grep, Bash or agents. Do not modify
the file.

1. Split todos into `in_progress` (listed separately at the end as "already in progress") and `open`.
2. Rank `open` todos with these criteria, in order of weight:
   1. dependencies: every id in `depends_on` must be absent from `todos[]`; todos with unsatisfied
      dependencies sink to the bottom (mention which ids block them);
   2. nature, judged from description and context: bug / crash / data loss > incorrect behaviour >
      UX annoyance > new feature > refactor / cleanup;
   3. size: smaller first (fewer plan steps when planned, fewer docs, narrower wording);
   4. age: older `created_at` first.
   Whether a todo has a plan or unanswered questions is **ignored** — `start` takes care of that.
3. Print at most 5 lines:
   ```
   1. #<id> — <description> — <why now, one clause>
   ```
   then, if any open todo has `plan == null`: `unplanned: #a, #b — run /todo plan`.
   If there are no open todos, print `nothing to start`.
