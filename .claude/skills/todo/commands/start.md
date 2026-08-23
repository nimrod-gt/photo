# /todo start <n>

Takes a todo into work. Requires a todo id. This is an interactive planning session: revalidate the
plan, resolve every open question with the user, produce a final execution plan in plan mode, then
execute it after approval.

## 1. Preconditions

- The id must exist.
- If `status == in_progress`: say so, show `branch`, and continue — the plan is revalidated anyway
  (that is how work is resumed).
- If `depends_on` contains ids still present in `todos[]`: warn, and ask (AskUserQuestion) whether to
  proceed anyway or stop.

## 2. Enter plan mode

If not already in plan mode: load `EnterPlanMode` with `ToolSearch select:EnterPlanMode` and call it.
From here until approval, the only file you may write is the plan file.

## 3. Make sure a plan exists

If `plan == null`, build one now following the "Agent brief" in `commands/plan.md` (read that file).
You may run it inline or via one `Plan` subagent; either way the result is the same JSON shape. Keep
the result in memory — `todo.json` is written in step 0 of the execution plan, not now.

## 4. Revalidate against the codebase

- `git log <plan.base_commit>..HEAD --oneline` and `git diff --stat <plan.base_commit>..HEAD` — what
  changed since the plan was written. If `base_commit` is unknown to git, treat the whole plan as
  stale and re-check every step.
- Read every file in `plan.steps[].files` and `docs`. Drop paths that no longer exist, adjust steps
  whose assumptions no longer hold, add steps that became necessary. Keep step granularity: each step
  independently reviewable, code builds after each.
- Re-read the project instruction file to pick up the build / lint / test commands (used in the
  checkpoints below). If it has none, derive them from the toolchain (go.mod → `go build ./... && go
  vet ./... && go test ./...`, package.json → its scripts, etc.).

## 5. Resolve questions

Collect every `plan.questions[]` entry with `answer == null` plus any new question that came up in
step 4. Ask them with `AskUserQuestion` (up to 4 per call; give concrete options, the first one being
your recommendation). Record each answer verbatim into that question's `answer`. Repeat until nothing
is open. If the answers change the steps, adjust them.

## 6. Write the execution plan (plan file)

Fixed skeleton — always these framing steps around the todo's own steps:

```
0. Persist: write the revised plan into todo.json — status "in_progress", branch
   "todo/<id>-<slug>", plan.revised_at = NOW, plan.base_commit = HEAD, all answers filled,
   updated_at = NOW. First action after approval, no further prompt.
1. Branch: `git checkout -b todo/<id>-<slug>` from the main branch (or `git checkout` the existing
   `branch` when resuming). Never work on main.
2..N. One entry per plan step (title, details, files), each followed by a CHECKPOINT:
     a. automatic review: run the project's build, lint and test commands; run /code-review on the
        diff of this step; fix what it finds;
     b. user review: stop, print what changed (files, behaviour, anything the user must try by
        hand), and wait for an explicit OK before the next step. Commit only when the user says so
        or the project rules allow it.
N+1. Final: full build/lint/test again, /code-review on the whole branch vs main, short summary of
     the branch.
N+2. `/todo finish <id>` (it asks its own confirmation).
```

`<slug>`: 2-4 lowercase words from the description, hyphen-separated, ASCII only.

Also list in the plan: the resolved questions with their answers, and the risks.

## 7. Approval

Call `ExitPlanMode`. After approval, execute steps 0 and 1 immediately, then proceed step by step,
honouring every checkpoint.
