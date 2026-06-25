# Test Database: Push E2E Fixtures

**Status:** ✅ Created
**Name:** `notion-sync-test-database-push`
**URL:** https://www.notion.so/35957008e88580c59e34f4191fd83907
**Database ID:** `35957008-e885-80c5-9e34-f4191fd83907`
**Data Source ID:** `35957008-e885-8068-9080-000b89086bb3`

Reproducible protocol for creating a Notion database dedicated to E2E testing of the v1.4.0 `push` redesign across all four phases. Used by `/test-push`.

## Created fixtures (page IDs)

| # | Name | Page ID | URL |
|---|---|---|---|
| 1 | Push: Canary | `35957008-e885-813a-886b-cbb6dd7c1598` | https://www.notion.so/35957008e885813a886bcbb6dd7c1598 |
| 2 | Push: Conflict Subject A | `35957008-e885-811d-ae4b-eb73607cc037` | https://www.notion.so/35957008e885811dae4beb73607cc037 |
| 3 | Push: Conflict Subject B | `35957008-e885-8141-9e44-ef7c58e4a487` | https://www.notion.so/35957008e88581419e44ef7c58e4a487 |
| 4 | Push: Formatting Fixture | `35957008-e885-8192-ab0f-c75e6a011b10` | https://www.notion.so/35957008e8858192ab0fc75e6a011b10 |
| 5 | Push: Cell-Level Test | `35957008-e885-815e-8e73-ea79c22f96d4` | https://www.notion.so/35957008e885815e8e73ea79c22f96d4 |
| 6 | Push: Soft Deleted | `35957008-e885-81e0-83c1-ff9fb4dbfda5` | https://www.notion.so/35957008e88581e083c1ff9fb4dbfda5 |
| 7 | Push: Null Edges | `35957008-e885-814f-9f19-c401d454b08d` | https://www.notion.so/35957008e885814f9f19c401d454b08d |
| 8 | Push: Rich-Text Roundtrip | `38a57008-e885-81c3-88c4-eec03393dcad` | https://www.notion.so/38a57008e88581c388c4eec03393dcad |

Relations wired: Page 1 → Page 4, Page 5 → Page 4 (both `Related` arrays contain Page 4's URL). Page 8 has no relation (clean round-trip workhorse).

## Why a separate DB

The single-data-source DB (`single-data-source/setup.md`) is shared with `/test-single-datasource-db` and exercises **import / refresh** — its rich-text content is not pinned for push verification. The v1.4.0 push redesign needs:

- **Phase 2** deliberately mutates pages into broken (stale-timestamp) states. Sharing a DB risks leaving rows in conflict states between runs.
- **Phase 3** verifies that **rich-text formatting on un-edited cells survives a push** (the original #55 symptom). This requires hand-curated, **stable** rich-text fixtures that no other skill ever pushes to.

Sharing breaks both. A dedicated DB is cheaper than retrofitting isolation later.

## Phase coverage

This DB is sized to cover all four phases of the v1.4.0 push redesign without renumbering or re-curating between phases.

| Phase | DAG nodes | Fixtures used | New rows needed? |
|---|---|---|---|
| 1: Confirmation gate | n12b → n13a | Page 1 (Canary) | No — covered today |
| 2: Validation halts | n21 → n22b | Pages 2, 3, 6, 7 | No |
| 3: Cell-level diff | n31 → n37 | Pages 4 (formatting), 5 (cell-level), 7 (nulls) | No |
| 3d: Rich-text un-skip | n31 rich_text re-include (#95/#99) | Pages 5 (workhorse), 8 (full supported round-trip) | **Yes — Page 8** |
| 4: Run summary JSON | n41 | All pages — exercises every status enum | No |

Total: **8 pages**, 1 schema, 1 self-relation. (Page 8 added for Phase 3d — live `highlight`/`==` coverage that Page 4 lacks.)

---

## Prerequisites

- Notion MCP tools available (`notion-create-database`, `notion-update-data-source`, `notion-create-pages`, `notion-update-page`, `notion-fetch`)
- A parent page in the user's workspace where the DB can live
- The Notion integration that owns `NOTION_SYNC_API_KEY` must be granted access to this DB

---

## Step 1: Create the database

```
Database name: notion-sync-test-database-push
```

Place under whichever parent page the user designates. Capture the returned `database_id` and `data_source_id` — fill them into the `Database ID` line at the top of this doc and into the `/test-push` skill's Step 2 import command.

## Step 2: Add property columns

Use `update-data-source` to add the schema in a single call. Schema is a **subset** of the single-source DB — only types `push` actually writes (no `people`, `files`, `formula`, `rollup`, etc.).

| Property Name | Type | Config |
|---|---|---|
| Name | title | (default) |
| Description | rich_text | `{}` |
| Score | number | `{"format": "number"}` |
| Category | select | Options: Research (blue), Engineering (green), Design (purple), Marketing (orange) |
| Tags | multi_select | Options: alpha (red), beta (blue), gamma (green), delta (gray) |
| Due Date | date | `{}` |
| Approved | checkbox | `{}` |
| Website | url | `{}` |
| Contact Email | email | `{}` |
| Phone | phone_number | `{}` |
| Related | relation | Self-relation: `{"data_source_id": "<this db's data_source_id>", "type": "single_property", "single_property": {}}` |

**Deliberately omitted** (push doesn't write these):
- `people`, `files`, `created_time`, `last_edited_time`, `created_by`, `last_edited_by`
- `formula`, `rollup`, `button`, `unique_id`, `verification`, `status`

## Step 3: Create the 7 fixture pages

Use `create-pages` with `parent: {"data_source_id": "<data_source_id>"}`. **All 7 pages in a single API call** so they share a creation timestamp.

### Page 1 — "Push: Canary" — phase 1 gate

**Purpose:** Single-page fixture for confirmation-gate cancel/proceed/dry-run paths. The skill's `Score` canary lives here.

```
Name:          Push: Canary
Description:   Phase 1 fixture — confirmation gate cancel/proceed/dry-run.
Score:         100
Category:      Research
Tags:          [alpha]
Due Date:      2026-06-01
Approved:      true
Website:       https://example.com/canary
Contact Email: canary@example.com
Phone:         +1-555-0001
```

**Body:** single paragraph: `Phase 1 canary fixture. Do not edit body — push never touches block content.`

### Page 2 — "Push: Conflict Subject A" — phase 2 halt aggregation

**Purpose:** Paired with Page 3. Used to verify multi-conflict aggregation halts the entire run (n22a). Its `notion-last-edited` will be staled locally to trigger n21d.

```
Name:          Push: Conflict Subject A
Description:   Phase 2 halt fixture A. Stale-timestamped during tests.
Score:         200
Category:      Engineering
Tags:          [beta]
Due Date:      2026-07-01
Approved:      false
Website:       https://example.com/halt-a
Contact Email: halt-a@example.com
Phone:         +1-555-0002
```

**Body:** single paragraph: `Phase 2 halt fixture A.`

### Page 3 — "Push: Conflict Subject B" — phase 2 halt aggregation

**Purpose:** Second halt subject. With Page 2, exercises "two halts → run-level halt with both reasons listed."

```
Name:          Push: Conflict Subject B
Description:   Phase 2 halt fixture B. Stale-timestamped during tests.
Score:         300
Category:      Design
Tags:          [gamma]
Due Date:      2026-07-15
Approved:      true
Website:       https://example.com/halt-b
Contact Email: halt-b@example.com
Phone:         +1-555-0003
```

**Body:** single paragraph: `Phase 2 halt fixture B.`

### Page 4 — "Push: Formatting Fixture" — phase 3 critical

**🚨 CRITICAL — DO NOT EDIT THIS PAGE'S TITLE OR DESCRIPTION FROM ANY SKILL.**

**Purpose:** Pin the original-#55 symptom. Has rich-text formatting in **both `Name` (title)** and **`Description` (rich_text)**. Phase 3 tests verify that pushing edits to **other fields** (Score, Category) does NOT clobber this formatting.

Created via Notion-flavored Markdown (the MCP tool parses these into rich-text annotations on save):

```
Name (markdown source — parses to rich text on save):
    Push: **Formatting** *Fixture* [anchor](https://example.com/anchor)

Description (markdown source — parses to rich text on save):
    **Bold here.** *Italic here.* `inline code` [link to xkcd](https://xkcd.com) ~~struck~~ Trailing plain text. $E = mc^2$

Score:         400
Category:      Marketing
Tags:          [alpha, delta]
Due Date:      2026-08-01
Approved:      true
Website:       https://example.com/fmt
Contact Email: fmt@example.com
Phone:         +1-555-0004
```

**Annotations stored on Notion (verified via fetch):** bold on `Formatting`, italic on `Fixture`, link on `anchor` → `https://example.com/anchor` (Name); bold on `Bold here.`, italic on `Italic here.`, inline code on `inline code`, link on `link to xkcd` → `https://xkcd.com`, strikethrough on `struck`, inline equation `E = mc^2` (Description).

**Body:** single paragraph: `Phase 3 formatting fixture. Title and Description have intentional rich-text formatting that other-field pushes must preserve.`

**Convention:** the `/test-push` skill's phase-3 section reads this page's `Name` and `Description` rich-text payload from Notion via `notion-fetch` BEFORE editing other fields, snapshots the structured payload (annotations array), pushes a `Score` change, then re-fetches and **diffs the structured payload** to verify zero drift in formatting. If a future fix breaks this, the test fails loudly.

### Page 5 — "Push: Cell-Level Test" — phase 3 single-cell diff

**Purpose:** Multiple non-formatting fields populated. Phase 3 picks ONE field to edit, pushes, and verifies only that field hit Notion's wire (others not in payload).

```
Name:          Push: Cell-Level Test
Description:   Phase 3 fixture — single-cell push verification.
Score:         500
Category:      Research
Tags:          [beta, gamma]
Due Date:      2026-09-01
Approved:      false
Website:       https://example.com/cell
Contact Email: cell@example.com
Phone:         +1-555-0005
```

**Body:** single paragraph: `Phase 3 cell-level fixture.`

### Page 6 — "Push: Soft Deleted" — phase 2 skip path

**Purpose:** Verify `notion-deleted: true` rows are skipped (n21b). Note: the soft-delete is a **frontmatter flag in the synced .md**, not a Notion-side state. This page lives normally on Notion; the test marks the local file's frontmatter.

```
Name:          Push: Soft Deleted
Description:   Phase 2 fixture — soft-delete skip path.
Score:         600
Category:      Engineering
Tags:          [delta]
Due Date:      2026-10-01
Approved:      false
Website:       https://example.com/deleted
Contact Email: deleted@example.com
Phone:         +1-555-0006
```

**Body:** single paragraph: `Phase 2 soft-delete fixture.`

### Page 7 — "Push: Null Edges" — phase 2/3/4 null handling

**Purpose:** Edge case for null/empty property handling on push. Validates that null `Due Date`, empty `Website`, etc. round-trip without producing spurious diffs or push errors.

```
Name:          Push: Null Edges
Description:   Phase 2/3 fixture — null and empty property handling.
Score:         (null)
Category:      (null)
Tags:          []
Due Date:      (null)
Approved:      false
Website:       (null)
Contact Email: (null)
Phone:         (null)
```

**Body:** single paragraph: `Phase 2/3 null-edges fixture.`

### Page 8 — "Push: Rich-Text Roundtrip" — phase 3d rich-text un-skip

**Purpose:** The positive counterpart to Page 4. Page 4 proves un-edited rich text *survives* a scalar push (read-only fixture); Page 8 proves edited rich text *round-trips through* a push with formatting intact. Carries the **6 push-supported** inline formats only — bold / italic / inline code / link / strikethrough / **highlight** — and deliberately **no equation** (Page 4 owns the equation-degradation case). Gives the live `highlight` (`==` / `yellow_background`) coverage Page 4 lacks.

Created via Notion MCP `create-pages` using **Notion-flavored Markdown** (note: the MCP dialect differs from notion-sync's `.md` dialect — see the dialect note below):

```
Name:          Push: Rich-Text Roundtrip   (plain — formatting lives in Description)
Description (MCP markdown source):
    **Bold run.** *Italic run.* `inline code` [link to xkcd](https://xkcd.com) ~~struck~~ <span color="yellow_bg">highlighted</span>
Score:         800
Category:      Research
Tags:          [alpha]
Due Date:      2026-12-01
Approved:      true
Website:       https://example.com/roundtrip
Contact Email: roundtrip@example.com
Phone:         +1-555-0008
```

**Annotations stored on Notion (verified via fetch 2026-06-25):** bold on `Bold run.`, italic on `Italic run.`, inline code on `inline code`, link on `link to xkcd` → `https://xkcd.com`, strikethrough on `struck`, **`yellow_background` on `highlighted`**.

**Body:** single paragraph: `Phase 3d rich-text round-trip fixture. Description carries the 6 push-supported formats (bold / italic / code / link / strikethrough / highlight). Do not edit body — push never touches block content.`

**🚨 The highlight MUST be `yellow_bg` (yellow_background), not any other background color.** notion-sync's importer maps **any** `*_background` color → `==text==` (`richtext.go:90`), but `ParseRichText` always emits `yellow_background` on push (`richtext_parser.go` — highlight has no color identity). So a non-yellow background would import as `==` and re-push as **yellow**, drifting the color on round-trip and failing R2's byte-identical annotation check. Non-yellow background collapse is a documented push limitation (Gap-1 family, alongside color / `@user` / equation).

**Dialect note (load-bearing for whoever recreates this page).** Two different inline-markdown dialects touch Page 8:
- **MCP create dialect** (used above to *create* the page): highlight = `<span color="yellow_bg">…</span>`, underline = `<span underline="true">…</span>`, equation = `` $`…`$ ``.
- **notion-sync `.md` dialect** (what `import` *writes locally* and what `push` *parses*): highlight = `==…==`, underline = `<u>…</u>`, link/bold/italic/code/strike identical.

Both resolve to the same Notion annotations, so the create-via-MCP page imports into notion-sync's dialect cleanly. The canonical local `Description` after a notion-sync `import` (the value R2/F1 compare against) is:

```
**Bold run.** *Italic run.* `inline code` [link to xkcd](https://xkcd.com) ~~struck~~ ==highlighted==
```

(Pin the exact string from a real `import` during the first `/test-push` run — Notion may re-segment; the fetch is the source of truth, this is the expected shape.)

**Convention:** the `R` step group snapshots Page 8's structured `Description` payload via `notion-fetch` before/after a push and asserts byte-identical annotation runs — the rich-text analog of Page 4's phase-3 check, but proving round-trip *through* a push rather than survival *beside* one.

---

## Step 4: Wire up relations

After all 7 pages exist, use `update-page` to set `Related` on a couple of pages so the relation property type has coverage:

- Page 1 (Canary) → relates to Page 4 (Formatting Fixture)
- Page 5 (Cell-Level) → relates to Page 4 (Formatting Fixture)

Phase 3 will use this to verify that pushing an unrelated field on Page 5 does NOT clobber the relation array. (Push currently sends every property — known issue. Phase 3's whole point.)

---

## Property coverage matrix

| Property Type | Populated | Null/Empty | Notes |
|---|---|---|---|
| title | All 8 | – | Page 4 has rich-text formatting in title; Page 8's title is plain |
| rich_text | 8 of 8 | – | Page 4 heavy (incl. equation, degrades on push); Page 8 the 6 push-supported formats; others plain |
| number | 7 of 8 | Page 7 null | |
| select | 7 of 8 | Page 7 null | All 4 options used at least once |
| multi_select | 7 of 8 | Page 7 empty `[]` | All 4 options used |
| date | 7 of 8 | Page 7 null | |
| checkbox | All 8 | – | true and false both represented |
| url | 7 of 8 | Page 7 null | |
| email | 7 of 8 | Page 7 null | |
| phone_number | 7 of 8 | Page 7 null | |
| relation | 2 of 8 | 6 empty | Page 1 → 4; Page 5 → 4 |

## Per-phase fixture map

| Fixture | Phase 1 | Phase 2 | Phase 3 | Phase 3d | Phase 4 |
|---|:---:|:---:|:---:|:---:|:---:|
| Page 1 — Canary | ✅ primary | – | – | – | ✅ pushed/no-op |
| Page 2 — Conflict A | – | ✅ halt subject | – | – | ✅ halted |
| Page 3 — Conflict B | – | ✅ halt subject | – | – | ✅ halted |
| Page 4 — Formatting | – | – | ✅ **untouched fixture** | ⚠️ C7 scalar-edit only; never `--force` | ✅ skippedNoOp |
| Page 5 — Cell-Level | – | – | ✅ primary | ✅ round-trip workhorse (R1/R3/R4/R5) | ✅ pushed |
| Page 6 — Soft Deleted | – | ✅ skip path | – | – | ✅ skippedNonRow |
| Page 7 — Null Edges | – | ✅ partial | ✅ null roundtrip | – | ✅ pushed/no-op |
| Page 8 — Rich-Text Roundtrip | – | – | – | ✅ primary (R2/R6) | ✅ pushed/no-op |

---

## Naming and identification convention

- Every page name starts with `Push:` so they're trivially distinguishable from any other DB or workspace content.
- Database name is `notion-sync push e2e fixtures` — long but unambiguous.
- All fake email addresses use `@example.com` (RFC 2606 reserved domain — never resolves).
- All phone numbers use the `+1-555-000N` test pattern (NANP test prefix — never dials).

## Things to NEVER do (regression hazards)

1. **Never edit Page 4's `Description` / title rich text locally, and never `--force` a folder containing Page 4.** Page 4's `Description` holds an inline equation (`$E = mc^2$`) that notion-sync's parser does **not** round-trip (#95 scoped equations out) — so re-serializing it on push degrades it to a literal `$…$` run. A `--force` push sends `changedFields=nil` (the whole row), which re-serializes `Description` → corruption. **One deliberate carve-out (post-#99):** a **scalar** edit on Page 4 under a **non-force** push (step C7) is safe — n33 sends only the changed scalar, leaving `Description` untouched — and C7 asserts exactly that survival. Everywhere else Page 4 stays read-only. (Pre-#99 the reason was "all rich_text corrupts"; #99's un-skip narrowed it to "equation/color/`@user` still degrade.")
2. **Never invent new `select` or `multi_select` options.** Notion auto-creates them on push and never garbage-collects. Stick to the 4+4 options spec'd above.
3. **Never push with `--force` against this DB unless the test specifically requires it.** Force masks conflict bugs (phase 2 needs real conflicts to test halts) **and** bypasses the per-cell diff, re-serializing every row's rich text — which corrupts Page 4's equation and collapses any non-yellow background. When a step does need `--force`, isolate the folder to a page whose rich text is safe to re-serialize (Page 8's 6 supported formats, or a plain-rich_text page) — **never Page 4**.
4. **Never edit Page 8's `Description` except through the `R` round-trip steps**, and keep its highlight `yellow_bg` (see the Page 8 fixture note — any other background collapses to yellow on push). Page 8 reverts cleanly because its formats are all push-supported; restore it via the step's revert or a re-import.
5. **Don't use any of these pages from `/test-single-datasource-db` or any other skill.** This DB is `/test-push`-only. Add a comment to other skills referencing this constraint if needed.

---

## State invariants for `/test-push`

The skill MUST ensure all 7 pages return to their original (Step 3) values after every run. The skill's Step F1 ("Final state verification") snapshots Notion via `notion-fetch` against the values documented in this file and fails the run if any drift remains.

If a run crashes mid-way and leaves drift:
- Re-importing into a fresh `test-output/push-e2e/` folder doesn't fix Notion-side drift — only local state
- Notion-side drift requires manual repair: open the page in Notion UI, restore the values per this doc
- Phase 4's run summary will help here — failed/halted entries point to which pages need attention

---

## Out of scope (intentionally)

- **`status` property** — can't create via API. Push currently doesn't write `status` either way (it's read-only in `pushSkipTypes`).
- **`people`, `files`** — push doesn't write these.
- **Block-level content** — push is frontmatter-only. Bodies are minimal placeholders.
- **Multi-data-source variants** — push contract is the same regardless. Multi-source coverage lives in `/test-double-datasource-db`.
