# /todo list

Uses only the injected `todo.json` (Read it only if injection failed). No other tools.

Print a table sorted by id:

```
#id  status       plan         branch              description
1    open         –            –                   Show a warning dialog when a folder has more than 10k photos
2    in_progress  ✓ 5 steps    todo/2-short-slug   ...
3    open         ✓ 3 steps ?2 –                   ...
```

- `plan`: `–` when `plan == null`; otherwise `✓ <n> steps`, plus ` ?<k>` when `k > 0` questions have
  `answer == null`.
- Keep descriptions on one line (no truncation unless wider than ~120 chars; then cut with `…`).
- Last line: `N todos: A open, B in progress, C unplanned` (unplanned = `plan == null`, any status).
- If `todos[]` is empty print `no todos — add one with /todo <text>`.

With `full` as argument, print each todo as a block instead of a row: id + description, then
`context`, `docs`, `depends_on`, `branch`, and for planned todos `plan.summary`, the list of step
titles and the open questions.
