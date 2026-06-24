# Rich text round-trip (import ⇄ push)

Design notes for how Notion rich text (the `title` property and every `rich_text`
"Text" column) moves between Notion and local Markdown — and why a faithful
push round-trip is hard. Read in this order:

| Note | Direction | Covers |
| --- | --- | --- |
| [`notion-to-markdown.md`](./notion-to-markdown.md) | **import** (Notion → Markdown) | `ConvertRichText` renders the structured rich-text array to a flat Markdown string; what it preserves, what it drops, and why rendering is not reversible. |
| [`push-migration-pains.md`](./push-migration-pains.md) | **push** (Markdown → Notion) | The missing inverse (parse Markdown back into rich-text structure), why true fidelity is two problems (missing inverse + lossy import), and v1.4.0's skip-don't-parse stance. |

**Related:** the inverse parser groundwork lives in
`internal/markdown/richtext_parser.go` (`ParseRichText`, unwired — see #95 Gap 2);
full round-trip fidelity is tracked in discussion #93; epic #55's 3a skips
`rich_text` for v1.4.0.
