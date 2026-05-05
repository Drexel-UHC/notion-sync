## Problem

On import, Notion `rich_text` and `title` properties are encoded as literal markdown via `markdown.ConvertRichText` — bold becomes `**bold**`, links become `[label](url)`, page mentions become `[[notion-id: …]]`, etc. On push (`internal/sync/push.go:buildPropertyValue`), that string is sent back to Notion as a single plain-text rich-text item with no parsing.

Result: any `rich_text` or `title` field with formatting in Notion gets silently flattened on the next push — bold disappears, mentions become literal `[[notion-id: …]]` text, hyperlinks become visible `[text](url)` characters in the title.

The conflict-detection guard does **not** catch this: the user really is the most recent editor, since `notion-last-edited` was updated by their last push. So a user editing one unrelated property (e.g., `Status`) and running `push` can silently re-flatten formatting on every other `rich_text`/`title` field they never touched.

## Why this is now larger

PR #52 adds `title` to the pushable set. Titles are far more visible than `rich_text` columns, and most databases have one. The roundtrip lossiness was already true for `rich_text` but is now load-bearing on every page.

## Proposed fix

In `buildPropertyPayload` (`internal/sync/push.go`), before constructing a payload entry for `rich_text` / `title`:

1. Reconstruct the plain-text equivalent of Notion's _current_ value (i.e., what `ConvertRichText` would emit if we re-imported now — requires fetching the current property value during the conflict check, or reusing the page already fetched there).
2. Compare to the local frontmatter value.
3. **Skip** the property if they match — i.e., only send updates for properties the user actually changed.

Side benefit: cuts API churn on push. Today, every pushable property is re-sent every time, even when untouched.

## Acceptance criteria

- A push run with no local edits to `rich_text`/`title` properties produces zero `UpdatePage` calls for those properties.
- A real edit to a title still pushes.
- A title imported with formatting (e.g., `**Bold**`) is **not** re-pushed unless the user changes the local string.

## Workaround until fixed

- Don't push databases that have formatted titles you don't intend to flatten.
- Or: edit titles locally to plain text before the first push, accepting the one-time flattening.

## Out of scope

True roundtrip preservation (parsing local markdown back into Notion's rich-text annotation tree) is a much bigger lift and not part of this issue.

## Priority

Low — the conflict guard prevents data loss in the destination when Notion has been edited; this is "subtle reformatting on roundtrip" rather than data corruption. Filing for visibility.

ref #51, follow-up to #52
