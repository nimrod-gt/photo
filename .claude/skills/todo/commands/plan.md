# /todo plan [n]

Writes `plan` for todos. Fully non-interactive: **never** ask the user anything here; every open
point becomes an entry in `plan.questions[]`.

## Targets

- `plan <n>`: that todo. If its `status` is `in_progress`, refuse: print
  `#<n> is in progress — use /todo start <n> to revise its plan` and stop. Otherwise (re)plan it,
  replacing any existing `plan` entirely.
- `plan` without id: every todo with `plan == null` and `status == open`. If there are none, print
  `all todos are planned` and stop.

## Run

For every target launch one planning subagent with the Agent tool, `subagent_type: "Plan"` (read-only),
all launches in a **single message** so they run in parallel. Each prompt is:

1. the full "Agent brief" section below, verbatim;
2. the todo as JSON (`id`, `description`, `context`, `docs`, `depends_on`);
3. the project root (absolute path) and `HEAD`;
4. the list of ids and descriptions of all **other** todos (so the agent can name dependencies).

Tell the user which todos are being planned, then wait for the agents.

## Merge

When an agent finishes: parse its returned JSON. If it is not valid JSON or lacks `steps`, retry that
agent once; on a second failure print the raw output and leave that todo unplanned.

For a valid result, Read `todo.json` again (fresh content), find the todo by id (it may have been
deleted meanwhile — then skip it and say so), and set:

```json
"plan": {
  "created_at": "<NOW>",
  "revised_at": null,
  "base_commit": "<HEAD>",
  "summary": "<agent summary>",
  "steps": [ {"title": "...", "details": "...", "files": ["..."]}, ... ],
  "questions": [ {"question": "...", "answer": null}, ... ],
  "risks": ["..."]
}
```

`depends_on` = union of the existing list and the agent's `depends_on` (ids that exist in `todos[]`
only, never the todo's own id). `updated_at = NOW`. Write the whole file. Print one line per todo:
`#<id> planned: <k> steps, <q> open questions`.

## Agent brief

You are writing an implementation plan for one micro-task of this project. Work alone; you cannot ask
anyone anything — every uncertainty, design choice the user should make, or missing information goes
into `questions`. Do not modify any file.

Procedure:

1. Read the project instruction file at the root (`CLAUDE.md`, `AGENTS.md` or similar) for
   conventions, layout, build/test commands.
2. Read every path in `docs`; follow references from there (callers, tests, related types) until you
   can name concrete files for every step. Explore enough to be specific, no more.
3. Produce 3-10 steps. Each step must be independently reviewable and leave the code building; name
   the files it touches (paths relative to the project root, existing or to-be-created). Where the
   project has tests, include adding/extending tests in the step that changes behaviour, not as a
   separate final step. Do not include "create branch", "review", "commit" or "finish" steps — the
   start command adds those around your steps.
4. Questions: phrase each so that a yes/no or a short answer settles it, and say which step it
   affects. Prefer proposing a default ("Assume X unless told otherwise") over blocking.
5. Risks: things that could break, hidden coupling, migration concerns. Short.
6. `depends_on`: ids from the "other todos" list that must be finished before this one (only when the
   dependency is real, e.g. same files or a prerequisite feature). Usually empty.

Return **only** a JSON object, no prose, no code fence:

```
{"summary": "one paragraph", "steps": [{"title": "...", "details": "...", "files": ["..."]}], "questions": ["..."], "risks": ["..."], "depends_on": []}
```

All text in English.
