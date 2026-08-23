# /todo new

Input: the todo text (everything after the `new` keyword, or the whole argument string when the keyword
was omitted). If the text is empty, print `nothing to add` and stop.

Tool budget for steps 2 and 3 together: at most 5 tool calls (Glob/Grep/short Reads; existence checks
count), in either branch of step 2.

## 1. Description

- Rewrite the text into one English sentence, imperative mood, concise (about 160 characters max).
- Minimal edits: keep the user's wording and intent, translate if needed, fix typos, grammar and
  punctuation. Do not invent scope that the user did not mention.
- If the user wrote a lot, the description stays one sentence; put the rest into `context`.

## 2. Context

Decide whether the current conversation is relevant to this todo:

- Relevant (the conversation touched the same feature, files or decision) → write 1-3 sentences a
  future agent needs to pick the task up quickly: decisions already made, where the relevant code
  lives, why the task exists. Usually no tool calls needed.
- Not relevant → quick orientation only, within the budget: enough to say which area/files the task
  concerns. If nothing useful turns up, `context` is `""`.

`context` is **not a plan**: no steps, no ordering, no "first do X then Y", no implementation choices
beyond what is already decided.

## 3. Docs

Up to 8 paths relative to the project root that an agent should read first: the files that own the
behaviour, their tests, the relevant spec/doc section. Find them within the budget; include only paths
that exist. Include `CLAUDE.md` (or equivalent) only if it documents this feature specifically.

## 4. Write

Read `todo.json` (create the default structure if missing), append to `todos[]`:

```json
{
  "id": <next_id>,
  "description": "...",
  "context": "...",
  "docs": [...],
  "depends_on": [],
  "status": "open",
  "branch": null,
  "created_at": "<NOW>",
  "updated_at": "<NOW>",
  "plan": null
}
```

Increment `next_id`, Write the whole file. Then print the created entry: `#<id>`, description,
context, docs. Nothing else; do not ask questions.
