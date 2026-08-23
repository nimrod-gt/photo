---
name: todo
description: Per-project micro-task list stored in todo.json. Subcommands: new (default, keyword optional), list, plan [n], start n, finish n, order, help.
argument-hint: "[<text> | new <text> | list [full] | plan [n] | start <n> | finish <n> | order | help]"
disable-model-invocation: true
allowed-tools: Read, Write, Glob, Grep, Agent, AskUserQuestion, Bash(git rev-parse:*), Bash(git log:*), Bash(git diff:*), Bash(git branch:*), Bash(git status:*)
---

# /todo

Micro-task tracker. Tasks live in `todo.json` at the project root (the directory Claude Code runs in).
This skill is project-independent: it contains no project-specific commands or paths. Project checks
(build/lint/test) are discovered at run time from the project's `CLAUDE.md` when a subcommand needs them.

## Injected context

- NOW (UTC): !`date -u +%Y-%m-%dT%H:%M:%SZ`
- HEAD: !`git rev-parse --short HEAD 2>/dev/null || echo none`
- SKILL_DIR: !`echo "${CLAUDE_SKILL_DIR}"`
- Current `todo.json` (empty default if the file does not exist yet):

!`cat todo.json 2>/dev/null || echo '{"version":1,"next_id":1,"todos":[]}'`

If any value above shows a placeholder instead of real output (shell execution disabled), obtain it
yourself: Read `todo.json`, run `git rev-parse --short HEAD`, and use the current UTC time.

## Dispatch

Arguments: `$ARGUMENTS`

Split on the first whitespace. Route by the first token (case-insensitive):

| first token | subcommand | rest of arguments |
|---|---|---|
| `list` | list | optional `full` |
| `plan` | plan | optional todo id |
| `start` | start | todo id (required) |
| `finish` | finish | todo id (required) |
| `order` | order | — |
| `help` | help | — |
| `new` | new | todo text |
| anything else | new | the **whole** argument string is the todo text |
| (empty) | help | — |

The `new` keyword is optional: `/todo fix empty filter crash` ≡ `/todo new fix empty filter crash`.

Then Read `SKILL_DIR/commands/<subcommand>.md` and follow it exactly. For `help`, print the usage block
at the bottom of this file and stop.

When a subcommand needs a todo id: it must parse as an integer and exist in `todos[]`; otherwise print
`todo #<n> not found` (or `missing todo id`) and stop without writing anything.

## Shared rules

- Everything written into `todo.json` is **English**. Translate user input if needed; fix typos and
  punctuation; otherwise keep the user's wording and intent. Reply to the user in the language they use.
- Write the **whole file** on every change: take the current content, apply the change, Write it back
  with 2-space indent, keys in the documented order, unknown extra keys preserved, trailing newline.
  Set `updated_at = NOW` on every todo you change. Never write from a subagent — only the main thread
  writes `todo.json`.
- Before a write that happens after a delay (e.g. after a background agent finishes), Read `todo.json`
  again and apply the change to the fresh content — the user may have added todos meanwhile.
- Do not ask the user anything unless the subcommand file says so (`start` and `finish` do; the rest
  never do).
- `list`, `order`, `finish` use no tools except what is needed to print and to write `todo.json`.
  They never read source code.

## todo.json format

```json
{
  "version": 1,
  "next_id": 3,
  "todos": [
    {
      "id": 1,
      "description": "Show a warning dialog when a folder has more than 10k photos",
      "context": "Short orientation for a future agent: decisions already made, where the relevant code lives, why. Not a plan.",
      "docs": ["path/relative/to/project/root.go", "another/file.md"],
      "depends_on": [],
      "status": "open",
      "branch": null,
      "created_at": "2026-08-23T10:12:00Z",
      "updated_at": "2026-08-23T10:12:00Z",
      "plan": null
    },
    {
      "id": 2,
      "description": "One-sentence imperative task description",
      "context": "",
      "docs": [],
      "depends_on": [1],
      "status": "in_progress",
      "branch": "todo/2-short-slug",
      "created_at": "2026-08-23T10:20:00Z",
      "updated_at": "2026-08-24T09:00:00Z",
      "plan": {
        "created_at": "2026-08-23T11:00:00Z",
        "revised_at": "2026-08-24T09:00:00Z",
        "base_commit": "afe0e73",
        "summary": "One-paragraph approach.",
        "steps": [
          {"title": "Short step title", "details": "What and how, 1-5 sentences.", "files": ["path/relative/to/root.go"]}
        ],
        "questions": [
          {"question": "Should the threshold be configurable?", "answer": null}
        ],
        "risks": ["What could go wrong or needs care."]
      }
    }
  ]
}
```

Rules:

- `id` is an integer taken from `next_id`, which then increments. Ids are never reused: `finish`
  deletes the entry but leaves `next_id` alone.
- `status` is `open` or `in_progress`. "Planned" is not a status: a todo is planned when `plan != null`.
- `description`: one sentence, imperative, concise (about 160 characters max). Extra detail goes into
  `context`, not into a longer description.
- `context`: at most ~3 sentences; may be `""`. Orientation only — never steps, never "first X then Y".
- `docs`: up to 8 paths relative to the project root, each verified to exist at write time.
- `depends_on`: ids of other todos that should be finished first. Optional; filled by `plan`/`start`
  when discovered. A dependency is satisfied when its id is no longer present in `todos[]`.
- `branch`: git branch for the work, set by `start` (`todo/<id>-<slug>`), `null` before that.
- `plan.base_commit`: short HEAD when the plan was written or last revised. `start` compares the
  codebase against it. `plan.revised_at` is `null` until `start` revises the plan.
- `plan.questions[]`: open points for the user; `answer` is `null` until `start` resolves it.
- Timestamps: ISO-8601 UTC (`YYYY-MM-DDTHH:MM:SSZ`), taken from NOW.

A JSON Schema for the file is in `SKILL_DIR/schema.json` (documentation, not enforced).

## Usage (printed by `help`)

```
/todo <text>          add a todo (same as /todo new <text>)
/todo new <text>      add a todo: English description + short context + related files
/todo list [full]     list todos (full: also context, docs, plan summary)
/todo plan [n]        write a plan for todo n, or for every todo without one (background agents, no questions)
/todo start <n>       take todo n into work: revalidate plan, resolve questions, plan mode, branch + reviews
/todo finish <n>      delete todo n after confirmation
/todo order           rank up to 5 todos worth starting next (reads only todo.json)
/todo help            this text
```
