# Legacy issue body — snapshot before reframing

Snapshot of issue #55's body prior to the 2026-05-06 reframe. The current issue body is now just a short epic header pointing at three sub-actions (document the flow, implement cell-level push, verify formatting survives). The detailed workstream / SOT / equality-semantics analysis below is preserved here for reference only.

---

# Epic: Push feature — design re-evaluation

**Status:** scratch / pre-issue
**Date:** 2026-05-05
**Trigger:** issue #55 (rich_text/title flattening) surfaced a deeper design gap — push was added without an explicit conflict / source-of-truth model, and operates at row granularity when the API supports cell granularity.

---

## TL;DR

Push works, but its design rests on assumptions that aren't documented and aren't always safe:

1. **No defined source-of-truth (SOT) model** when local and Notion both drift — push trusts local frontmatter wholesale, with only a `last_edited_time` guard.
2. **Row-level (page-level) granularity** in practice — every pushable property is re-sent on every push, even untouched ones. The API supports cell-level updates; we don't use that.
3. **Lossy round-trip** for `rich_text` / `title` — the obvious symptom of (1) + (2). Issue #55.

These are not three independent bugs. They're one architectural shape (push-without-diff) viewed from three angles. The epic is to redesign push around an explicit SOT + diff model and treat #55 as one downstream fix that falls out for free.

---

## Why now

- PR #52 made `title` pushable. Titles are visible on every page; the lossy round-trip is no longer a corner case in obscure rich-text columns.
- Push is graduating from "occasionally used" to a real workflow. The cost of the current shortcuts is climbing.
- The same fix mechanism (per-property diff vs. Notion's current value) addresses #55, the API-churn problem, and provides the infrastructure for a documented SOT model.

---

## Workstreams

### WS1 — Conflict model & source-of-truth doctrine *(design + docs, no code yet)*

**Question:** when a user runs `push`, what is the contract?

Today's behavior, undocumented:

- Read local frontmatter as the desired state.
- Fetch Notion page, compare `last_edited_time` against local `notion-last-edited`.
- If timestamps match → push everything pushable.
- If timestamps differ → flag conflict, skip page.

Open questions to answer:

- **Should `push` auto-refresh first?** Right now a stale local file pushes happily as long as nothing changed in Notion since the last sync. But "nothing changed in Notion" is a one-shot check at push time — between fetching state and posting state, Notion can drift, and we don't re-check. Should push do an implicit `refresh` on the affected pages first so the user is diffing against current Notion state?
- **What is SOT when both sides have moved?** The conflict guard says "Notion is SOT, bail out." Is that the right default? Should there be a `--prefer-local` / `--prefer-remote` switch? Today there's just `--force`, which means "pretend there's no conflict," not "I've considered the diff and choose local."
- **What's the user's mental model when state is fluid?** Documents need to make explicit:
  - Which file does the user edit? (markdown, frontmatter, or both?)
  - When push happens, what gets sent? (today: all pushable props, regardless of edit)
  - When conflict happens, what's the recovery path? (today: re-import overwrites local, no merge)
- **Do we surface a diff before pushing?** A `--dry-run` exists but reports counts, not what changed at the property level. Should it show the diff?

**Deliverable:** a short design doc (could live in `docs/`) that names the SOT model push assumes, the failure modes, and the user-facing contract. No code.

---

### WS2 — Cell-level push (per-property diff) *(architectural fix)*

**The change:** before sending an `UpdatePage` payload, diff each property against Notion's current value and only include properties that actually changed locally.

**Mechanism:**

- The `notionPage` is already fetched during the conflict check (`push.go:101`). Reuse it.
- For each pushable property:
  - Convert Notion's current value to its frontmatter-equivalent form (using the same encoders that import uses — `ConvertRichText`, the property mapper in `page.go`).
  - Compare to the local frontmatter value with type-appropriate equality (string compare for text, set compare for `multi_select`, etc.).
  - Equal → omit from payload. Different → include.
- If the resulting payload is empty → skip the `UpdatePage` call entirely (currently treated as `Skipped`).

**What this buys:**

- Fixes #55 as a special case — `rich_text`/`title` don't get re-sent unless the local string actually differs from Notion's re-encoded current value, so no flattening of untouched fields.
- Stops all the secondary effects of no-op writes:
  - No more spurious `last_edited_time` bumps from idempotent updates.
  - No more Notion automations / formula recomputes firing on no-op pushes.
  - Edit history stops getting polluted with the user as last editor on rows they never touched.
  - API quota stops being burned on no-ops (relevant given the ~3 req/s throttle).
- Reduces the blast radius of bugs in any single property type's encoder — only properties the user touched get exercised on the push path.

**Equality semantics, per type** (rough sketch):

| Type | Notion side | Local side | Compare |
|---|---|---|---|
| `title`, `rich_text` | `ConvertRichText(prop.X)` | string | `==` |
| `select`, `status` | `prop.Select.Name` (or empty) | string or `nil` | nil-safe `==` |
| `multi_select` | sorted names | sorted strings | slice eq |
| `number` | `*float64` | `float64` or `nil` | nil-safe `==` |
| `checkbox` | `bool` | `bool` | `==` |
| `date` | reformatted `start [→ end]` | string | `==` |
| `url`, `email`, `phone_number` | string or `nil` | string or `nil` | nil-safe `==` |
| `relation` | sorted IDs | sorted strings | slice eq |

Each is small. Total diff logic is probably ~80–120 LOC plus tests.

**Acceptance:**

- Push run on a folder with zero local edits → zero `UpdatePage` calls (today: N calls, one per row).
- Push run editing only `Status` on one row → exactly one `UpdatePage` call carrying only the `status` property.
- Imported title with `**Bold**` formatting, no local edit → not re-pushed; Notion's bold survives.
- Real edits still push (regression test for the obvious case).

---

### WS3 — Issue #55 (rich_text/title flattening) *(absorbed into WS2)*

The narrow fix proposed in `scope-richtext-title.md` is the WS2 mechanism applied only to `rich_text` and `title`. Once WS2 lands, #55 is fixed automatically. Recommend converting #55 into this epic (retitle + rebody) and using `scope-richtext-title.md` as the WS2 acceptance canary.

---

## Suggested issue layout

If the epic is filed as a parent issue, child issues roughly:

1. **Design doc — push conflict model & SOT** (WS1, no code)
2. **Per-property diff in push** (WS2, the actual fix)
3. ~~#55~~ — close as superseded once #2 lands, or keep open as the canary acceptance criterion for #2

Sequencing: WS1 first if there's any chance the SOT decisions change WS2's contract (e.g., should push auto-refresh first? — that affects what "Notion's current value" means in the diff). If WS1 is going to take a while and the team is comfortable with the existing implicit model, WS2 can ship in parallel and the doc can be retroactive.

---

## Out of scope (still)

- **True rich-text round-trip** (parsing local markdown back into Notion's annotation tree). Bigger lift, separate epic. WS2 only stops *unintended* flattening; intentionally edited markdown formatting is still flattened on push.
- **Body content push.** Push only touches frontmatter properties. Page bodies are import-only. No change here.
- **Three-way merge UI** for conflicts. The `--prefer-local` / `--prefer-remote` discussion in WS1 is about defaults and switches, not about a merge tool.

---

## Open questions for grilling

- Is `push` a workflow we want to invest in, or a convenience feature? The answer changes the bar for WS1.
- Do we have any user reports of the no-op `last_edited_time` churn breaking conflict detection on subsequent runs? (If yes, WS2 priority bumps.)
- Should the diff be visible to the user (a `push --diff` mode) before they commit to the push? Cheap to add once WS2 lands.

---

## References

- `internal/sync/push.go` — current push implementation
- `internal/sync/page.go:265+` — property → frontmatter encoder (mirror for the "Notion side" of the diff)
- `internal/markdown/richtext.go` — `ConvertRichText` (the encoder for the lossy types)
- Issue #55 (this folder: `scope-richtext-title.md`) — the visible-symptom issue that motivated this epic
- Issue #51 — title-property exclusion review (closed)
- PR #52 — added title to pushable set
