---
name: todo
description: Per-project micro-task list stored in todo.json. Subcommands: new (default, keyword optional), list, plan [n], start n, finish n, order, help.
argument-hint: "[<text> | new <text> | list [full] | plan [n] | start <n> | finish <n> | order | help]"
allowed-tools: Read, Write, Glob, Grep, Agent, AskUserQuestion, ToolSearch, Bash(date:*), Bash(git rev-parse:*), Bash(git log:*), Bash(git diff:*), Bash(git branch:*), Bash(git status:*), Bash(git checkout:*), Bash(git symbolic-ref:*)
---

# /todo

Micro-task tracker. Tasks live in `todo.json` in the **current working directory** (not the git
toplevel) — run `/todo` from the project root so `docs` paths stay root-relative.
This skill is project-independent: it contains no project-specific commands or paths. Project checks
(build/lint/test) are discovered at run time from the project's `CLAUDE.md` when a subcommand needs them.
`allowed-tools` above covers the skill's own needs; `start` additionally runs the project's build/test
commands and plan-mode tools, which may prompt for permission.

## Injected context

- NOW (UTC): !`date -u +%Y-%m-%dT%H:%M:%SZ`
- HEAD: !`git rev-parse --short HEAD 2>/dev/null || echo none`
- SKILL_DIR: !`echo "${CLAUDE_SKILL_DIR}"`
- Current `todo.json` (empty default if the file does not exist yet):

!`cat todo.json 2>/dev/null || echo '{"version":1,"next_id":1,"todos":[]}'`

Fallbacks:

- If a value above shows a placeholder instead of real output (shell execution disabled): Read
  `todo.json`, run `date -u +%Y-%m-%dT%H:%M:%SZ` and `git rev-parse --short HEAD` yourself.
- If SKILL_DIR is empty: it is `.claude/skills/todo` under the project root.
- If the injected `todo.json` content is empty (0-byte file): treat it as the default above.
- If it is not valid JSON: print `todo.json is not valid JSON — fix it by hand` and stop. Never
  overwrite a file you cannot parse.
- HEAD `none` means no git repository; `plan.base_commit` then stores `none`.

## Dispatch

Arguments: `$ARGUMENTS`

Split on the first whitespace into `first` and `rest`. Route (first token case-insensitive):

| condition | subcommand | argument |
|---|---|---|
| empty `$ARGUMENTS` | help | — |
| `help`, rest empty | help | — |
| `order`, rest empty | order | — |
| `list`, rest empty or `full` | list | `full` flag |
| `plan`, rest empty or an integer | plan | optional id |
| `start`, rest is an integer | start | id |
| `finish`, rest is an integer | finish | id |
| `start` / `finish`, rest empty | — | print `missing todo id`, stop |
| `new` | new | rest is the todo text |
| anything else | new | the **whole** `$ARGUMENTS` is the todo text |

The `new` keyword is optional: `/todo fix empty filter crash` ≡ `/todo new fix empty filter crash`.
Natural text that merely starts with a keyword is still a todo: `/todo Start the importer rewrite`
→ `new`, because `the importer rewrite` is not an integer.

Then Read `SKILL_DIR/commands/<subcommand>.md` and follow it exactly. For `help`, print the usage block
at the bottom of this file and stop.

When a subcommand gets an id that is not present in `todos[]`: print `todo #<n> not found` and stop
without writing anything.

## Shared rules

- Everything written into `todo.json` is **English**. Translate user input if needed; fix typos and
  punctuation; otherwise keep the user's wording and intent. Reply to the user in the language they use.
- Write the **whole file** on every change, 2-space indent, keys in the documented order, unknown
  extra keys preserved, trailing newline. **Always Read `todo.json` immediately before every Write**
  (the injected content is for reading; a Write must be based on a fresh Read — other sessions or a
  `new` run meanwhile may have changed the file). Never write from a subagent — only the main thread
  writes `todo.json`.
- Timestamps: NOW is valid for writes that happen right away. For a write after background agents
  or user interaction, run `date -u +%Y-%m-%dT%H:%M:%SZ` again and use that value. Set `updated_at`
  on every todo you change.
- Do not ask the user anything unless the subcommand file says so (`start` and `finish` do; the rest
  never do).
- `list` and `order` read nothing but the injected `todo.json` (plus their command file, plus
  `todo.json` itself if injection failed). `finish` additionally asks one question and writes.
  None of them reads source code, and neither does the main thread in `new` — `new` and `plan`
  delegate all code exploration to background agents.

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

- Every key above is always present (`depends_on` may be `[]`, `branch`/`plan` may be `null`).
- `id` is an integer taken from `next_id`, which then increments. Ids are never reused: `finish`
  deletes the entry but leaves `next_id` alone.
- `status` is `open` or `in_progress`. "Planned" is not a status: a todo is planned when `plan != null`.
- `description`: one sentence, imperative, concise (about 160 characters max). Extra detail goes into
  `context`, not into a longer description.
- `context`: at most ~3 sentences; may be `""`. Orientation only — never steps, never "first X then Y".
- `docs`: up to 8 paths relative to the project root, each verified to exist at write time.
- `depends_on`: ids of other todos that should be finished first; filled by `plan`/`start` when
  discovered. A dependency is satisfied when its id is no longer present in `todos[]`.
- `branch`: git branch for the work, set by `start` (`todo/<id>-<slug>`), `null` before that and in
  projects without git.
- `plan.base_commit`: short HEAD when the plan was written or last revised (`none` without git).
  `start` compares the codebase against it. `plan.revised_at` is `null` until `start` revises the plan.
- `plan.questions[]`: open points for the user; `answer` is `null` until `start` resolves it.
- Timestamps: ISO-8601 UTC (`YYYY-MM-DDTHH:MM:SSZ`).

A JSON Schema for the file is in `SKILL_DIR/schema.json` (documentation, not enforced).

## Usage (printed by `help`)

```
/todo <text>          add a todo (same as /todo new <text>)
/todo new <text>      add a todo: English description + short context + related files (background agent)
/todo list [full]     list todos (full: also context, docs, plan summary)
/todo plan [n]        write a plan for todo n, or for every todo without one (background agents, no questions)
/todo start <n>       take todo n into work: revalidate plan, resolve questions, plan mode, branch + reviews
/todo finish <n>      delete todo n after confirmation
/todo order           rank up to 5 todos worth starting next (reads only todo.json)
/todo help            this text
```
