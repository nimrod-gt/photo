# /todo new

Input: the todo text (everything after the `new` keyword, or the whole argument string when the keyword
was omitted). If the text is empty, print `nothing to add` and stop.

The main thread does **no** code exploration itself: `context` and `docs` come from one background
subagent. The main thread only rewrites the description, launches the agent, and writes `todo.json`.

## 1. Description

- Rewrite the text into one English sentence, imperative mood, concise (about 160 characters max).
- Minimal edits: keep the user's wording and intent, translate if needed, fix typos, grammar and
  punctuation. Do not invent scope that the user did not mention.
- If the user wrote a lot, the description stays one sentence; the rest goes to the agent as
  conversation context.

## 2. Research agent

Launch one subagent with the Agent tool, `subagent_type: "Explore"` (read-only). It runs in the
background; say `researching #<next_id>: <description>` and wait for it. The prompt is:

1. the full "Agent brief" section below, verbatim;
2. the description from step 1;
3. the project root (absolute path);
4. what the **current conversation** already established about this task, if anything: decisions
   already made, files already touched, why the task exists. If the conversation is unrelated to the
   todo, say so in one line instead.

If the agent returns invalid JSON, relaunch it once with the same prompt; on a second failure use
`context` from the conversation (or `""`) and `docs: []`, and say the research failed.

## 3. Write

Run `date -u +%Y-%m-%dT%H:%M:%SZ` (fresh time — the agent ran meanwhile), Read `todo.json` (create
the default structure if missing), append to `todos[]`:

```json
{
  "id": <next_id>,
  "description": "...",
  "context": "<agent context>",
  "docs": [<agent docs>],
  "depends_on": [],
  "status": "open",
  "branch": null,
  "created_at": "<fresh time>",
  "updated_at": "<fresh time>",
  "plan": null
}
```

Drop any path the agent returned that does not exist. Increment `next_id`, Write the whole file. Then
print the created entry: `#<id>`, description, context, docs. Nothing else; do not ask questions.

## Agent brief

You are collecting orientation for one micro-task of this project so that a future agent can pick it
up quickly. Do not modify any file. Budget: at most 8 tool calls (Glob/Grep/short Reads; existence
checks count) — enough to name the area and the files that own the behaviour, no more.

Produce two things:

1. `context`: at most 3 sentences a future agent needs — decisions already made, where the relevant
   code lives, why the task exists. Fold in whatever the conversation already established; it
   outranks anything you infer from the code. If nothing useful turns up, `""`.
   `context` is **not a plan**: no steps, no ordering, no "first do X then Y", no implementation
   choices beyond what is already decided.
2. `docs`: up to 8 paths **relative to the project root** that an agent should read first — the files
   that own the behaviour, their tests, the relevant spec/doc section. Only paths you verified exist.
   Include the project instruction file (`CLAUDE.md` or equivalent) only if it documents this feature
   specifically.

Return **only** a JSON object, no prose, no code fence:

```
{"context": "...", "docs": ["path/relative/to/root.go"]}
```

All text in English.
