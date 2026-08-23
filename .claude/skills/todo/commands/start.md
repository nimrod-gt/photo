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

Load both tools with `ToolSearch select:EnterPlanMode,ExitPlanMode`, then call `EnterPlanMode`
(harmless if plan mode is already active). From here until approval, the only file you may write is
the plan file.

## 3. Make sure a plan exists

If `plan == null`, build one now following the "Agent brief" in `SKILL_DIR/commands/plan.md` (Read
that file). You may run it inline or via one `Plan` subagent; either way the result is the same JSON
shape. Keep the result in memory — `todo.json` is written in step 1 of the execution plan, not now.

## 4. Revalidate against the codebase

- `git log <plan.base_commit>..HEAD --oneline` and `git diff --stat <plan.base_commit>..HEAD` — what
  changed since the plan was written. If `base_commit` is `none` or unknown to git, treat the whole
  plan as stale and re-check every step. Without a git repository skip the git parts.
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

Fixed skeleton — always these framing steps around the todo's own steps. Record the absolute
`SKILL_DIR` in the plan file (the last step needs it).

```
0. Branch (skip without git; `branch` stays null): if `branch` is already set, `git checkout <branch>`.
   Otherwise detect the default branch (`git symbolic-ref --short refs/remotes/origin/HEAD`, else
   `main`, else `master`), `git checkout <default>`, then `git checkout -b todo/<id>-<slug>`.
   Never work on the default branch.
1. Persist: run `date -u +%Y-%m-%dT%H:%M:%SZ` and `git rev-parse --short HEAD`, Read todo.json,
   write the revised plan — status "in_progress", branch (keep the existing value if set, else the
   one created in step 0), plan.revised_at and updated_at = fresh time, plan.base_commit = fresh HEAD,
   all answers filled. No further prompt.
2..N. One entry per plan step (title, details, files), each followed by a CHECKPOINT:
     a. automatic review: run the project's build, lint and test commands; run /code-review on the
        diff of this step if that skill is available, otherwise review the diff yourself with the
        same rigour; fix what it finds;
     b. user review: stop, print what changed (files, behaviour, anything the user must try by
        hand), and wait for an explicit OK before the next step. Commit only when the user says so
        or the project rules allow it.
N+1. Final: full build/lint/test again, /code-review (or own review) of the whole branch vs the
     default branch, short summary of the branch.
N+2. Finish: Read `<SKILL_DIR>/commands/finish.md` and follow it for #<id> (it asks its own
     confirmation). The model cannot invoke /todo itself; if reading the file is impossible, tell the
     user to run `/todo finish <id>`.
```

`<slug>`: 2-4 lowercase words from the description, hyphen-separated, ASCII only.

Also list in the plan: the resolved questions with their answers, and the risks.

## 7. Approval

Call `ExitPlanMode`. After approval, execute steps 0 and 1 immediately, then proceed step by step,
honouring every checkpoint.
