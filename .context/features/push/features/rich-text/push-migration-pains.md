# Rich text: Markdown → Notion (push direction) — migration pains

**Status:** documented (current behavior + open problem)
**Code:** `internal/sync/push.go` — `buildPropertyValue` (`rich_text` / `title` cases)
**Related:** import side → `notion-to-markdown.md`; full fix → discussion #93; epic #55 (3a skips `rich_text`)

## 1. Overview

Importing rich text mostly works: Notion's structured rich text **renders** into a flat
Markdown string (see `notion-to-markdown.md`). Pushing is the hard direction — to write a
local Markdown string back to Notion *and preserve the formatting*, we need the **inverse**
operation: read `**bold**` and rebuild Notion's `{ text:"bold", annotations:{ bold:true } }`
structure. That inverse — call it **reverse serialization** (parse / deserialize-from-Markdown)
— **does not exist** in the codebase.

### The round-trip, as two halves

```
Notion rich text  ──render (have it)──▶  markdown string   ✅  ConvertRichText
  {bold:true}                              **bold**

markdown string   ──parse (MISSING)──▶  Notion rich text   ❌  no inverse exists
  **bold**                                 {bold:true}
```

- **Import = render / serialize-to-Markdown.** Structure → text. We have this.
- **Push = parse / deserialize-from-Markdown.** Text → structure. We do **not** have this.
  Today `buildPropertyValue` stuffs the raw string `**bold**` into a plain-text content
  field, so Notion stores the literal asterisks — formatting destroyed, markers visible.

### The catch: it is two problems, not one

This is why true fidelity is "roadmap, never 100%" (discussion #93):

| | Problem | Fix |
| --- | --- | --- |
| **A** | **The missing inverse.** Even formatting that *survived* import (`**bold**`) has no code to turn it back into Notion structure. | Write a Markdown-inline → rich-text parser. Alone, this kills the literal-`**`-junk corruption. |
| **B** | **Import was already lossy.** Some data never reached the `.md` — text colors, user-mention identity (see the "drops" table in `notion-to-markdown.md`). | No parser can rebuild what is not in the file. Requires an **import-side** change — e.g. stash the raw rich-text JSON alongside the rendered Markdown. |

True round-trip fidelity = **A + B**. The corruption seen *today* is pure **A** — we push
Markdown raw, with no parser.

### v1.4.0 stance: skip, do not parse

Epic #55's 3a **skips** `rich_text` (and shares the decision for `title`): it is not diffed
and not pushed. That sidesteps both A and B for v1.4.0 — nothing to parse, nothing to
corrupt — at the cost of leaving Text columns push-read-only. The full A + B fix is tracked
in discussion #93 as a future release.
