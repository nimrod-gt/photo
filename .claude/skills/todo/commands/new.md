# /todo new

Input: the todo text (everything after the `new` keyword, or the whole argument string when the keyword
was omitted). If the text is empty, print `nothing to add` and stop.

## 1. Description

- Rewrite the text into one English sentence, imperative mood, concise (about 160 characters max).
- Minimal edits: keep the user's wording and intent, translate if needed, fix typos, grammar and
  punctuation. Do not invent scope that the user did not mention.
- If the user wrote a lot, the description stays one sentence; put the rest into `context`.

## 2. Context

Decide whether the current conversation is relevant to this todo:

- Relevant (the conversation touched the same feature, files or decision) → write 1-3 sentences a
  future agent needs to pick the task up quickly: decisions already made, where the relevant code
  lives, why the task exists.
- Not relevant → do a quick orientation only: at most 5 tool calls (Glob/Grep, short Reads of the
  project instruction file), enough to say which area/files the task concerns. If nothing useful turns
  up, `context` is `""`.

`context` is **not a plan**: no steps, no ordering, no "first do X then Y", no implementation choices
beyond what is already decided.

## 3. Docs

Up to 8 paths relative to the project root that an agent should read first: the files that own the
behaviour, their tests, the relevant spec/doc section. Find them with Glob/Grep inside the same
5-tool-call budget; include only paths that exist. Include `CLAUDE.md` (or equivalent) only if it
documents this feature specifically.

## 4. Write

Append to `todos[]`:

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

Increment `next_id`, Write the whole file (create it if missing). Then print the created entry:
`#<id>`, description, context, docs. Nothing else; do not ask questions.
