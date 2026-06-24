# Rich text: Notion → Markdown (import direction)

**Status:** documented (current behavior)
**Code:** `internal/markdown/richtext.go` — `ConvertRichText`
**Related:** push round-trip fidelity → discussion #93; epic #55 (3a skips `rich_text`)

## 1. Overview

`ConvertRichText` is the **import-side renderer**. On `import` / `refresh`, Notion returns
a property's value as a **structured array** of segments — each segment is
`{ text, annotations }` where annotations = bold / italic / code / strikethrough /
underline / color, plus optionally a link, or the segment is a mention / equation.
`ConvertRichText` walks that array and **renders it to a flat Markdown string** that gets
saved into the `.md` frontmatter.

Two property types flow through this path: the `title` property and every `rich_text`
("Text") column. Both store the same annotated-text structure, so both share this
renderer — and its limitations.

### What it preserves (rendered as Markdown syntax characters)

| Notion annotation | Rendered to | Notes |
| --- | --- | --- |
| bold | `**text**` | delimiters placed at annotation boundaries to avoid garbled asterisks across adjacent segments |
| italic | `*text*` | |
| code | `` `text` `` | |
| strikethrough | `~~text~~` | |
| underline | `<u>text</u>` | HTML — Markdown has no underline |
| background color | `==text==` | highlight |
| link | `[text](url)` | |
| equation | `$expr$` | |
| page / database mention | `[[notion-id: …]]` | object id kept |
| date mention | `start` or `start → end` | |

### What it drops (lossy — the root of the round-trip problem)

| Notion data | Outcome |
| --- | --- |
| foreground / text color | **dropped** — only `*_background` colors survive (as `==…==`); a colored *text* run keeps its text, loses its color |
| user mention | collapses to `@name` — the **user id is lost** |
| (any annotation Notion adds later) | silently dropped unless mapped here |

### The key nuance

It **renders**; it does not **serialize reversibly**. Two consequences:

1. **Some data is gone for good** — text colors, mention identity. Not in the local file
   at all, so no downstream process (including `push`) can reconstruct it.
2. **There is no inverse function.** Nothing parses `**bold**` *back* into Notion's
   `{ bold: true }` structure. The "parse" half (Markdown → rich-text array) does not
   exist — import only does the "render" half.

### Concrete trace

A Notion cell containing the word **bold** (bold annotation):

```
Notion:  [{ text:{content:"bold"}, annotations:{bold:true} }]   ← structure
   │ ConvertRichText (import)
   ▼
.md:     **bold**                                                ← 8 plain characters
```

Once it is `**bold**` in the file, the "boldness" lives in the asterisks, not in any
structure. Going the other direction (Markdown → Notion structure) is the parser we do
**not** have — that gap is why naive `push` re-sends `**bold**` as literal plain text and
corrupts the cell, and why epic #55's 3a **skips** `rich_text` for v1.4.0. The full
round-trip fix is tracked in discussion #93.
