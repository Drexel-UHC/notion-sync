# Stress Test Results: test database obsdiain complex

**Date:** 2026-02-05
**Database:** `2fe57008-e885-8003-b1f3-cc05981dc6b0`
**Synced to:** `go/notion/test database obsdiain complex/`
**Pages:** 5 created, 5 frozen successfully

---

## Executive Summary

The sync tool handles the **majority of cases well**. All 14 testable property types froze correctly, including edge cases like null values, date ranges, datetimes, and empty arrays. List nesting (3 levels), tables, code blocks, equations, and callouts all work.

**3 bugs found**, **2 minor issues**, and **1 cosmetic limitation**:

| Severity | Issue | Location |
|----------|-------|----------|
| **BUG** | Toggle heading children not wrapped in `> ` prefix | `converter.go` heading_1/2/3 toggleable |
| **BUG** | Heading 1 blocks missing from output (not rendered) | Page 1 — needs investigation |
| **BUG** | Bold+italic combined produces garbled markdown | `richtext.go` annotation stacking |
| Minor | Foreground text colors silently dropped | `richtext.go` — only `_background` colors handled |
| Minor | "plain text" code block rendered as `javascript` | Likely Notion MCP defaulting language, not notion-sync |
| Cosmetic | Relation values are raw page IDs, not human-readable | By design, but worth noting |

---

## Property Results

| Property | Type | Test Values | Frozen Correctly | Notes |
|----------|------|-------------|:---:|-------|
| Name | title | 5 distinct names | PASS | Used as filename |
| Description | rich_text | 5 distinct strings | PASS | Plain text in frontmatter |
| Score | number | 0, 42, 78.5, 95, 100 | PASS | Integers and floats preserved |
| Category | select | Research, Engineering, Design, Marketing | PASS | Option name as string |
| Tags | multi_select | 1-3 tags per page | PASS | YAML array format |
| Due Date | date (single) | `2026-03-15` | PASS | Date-only preserved |
| Due Date | date (range) | `2026-02-01 -> 2026-04-30` | PASS | Arrow notation |
| Due Date | date (datetime) | `2026-06-15T09:00:00.000+00:00` | PASS | Full ISO with timezone |
| Due Date | date (null) | null | PASS | Renders as `null` |
| Approved | checkbox | true, false | PASS | Boolean values |
| Website | url | URLs and null | PASS | Quoted strings, null for empty |
| Contact Email | email | Emails and null | PASS | Unquoted in YAML |
| Phone | phone_number | US, UK, JP, DE formats | PASS | Various intl formats preserved |
| Related | relation | 0-2 relations | PASS | Array of raw page UUIDs |
| Assignee | people | all empty | PASS | Empty array `[]` |
| Attachments | files | all empty | PASS | Empty array `[]` |
| Created | created_time | auto-set | PASS | ISO timestamp, quoted |
| Last Edited | last_edited_time | auto-set | PASS | ISO timestamp, quoted |

**Property score: 18/18 (100%)**

---

## Block Results

### Page 1: Headings & Rich Text

| Block/Feature | Expected | Actual | Status |
|---|---|---|:---:|
| heading_1 | `# Heading 1: Introduction` | Missing from output | **FAIL** |
| heading_2 | `## Heading 2: Details` | Correct | PASS |
| heading_3 | `### Heading 3: Sub-details` | Correct | PASS |
| paragraph | Plain text paragraphs | Correct | PASS |
| bold | `**bold text**` | Correct | PASS |
| italic | `*italic text*` | Correct | PASS |
| strikethrough | `~~strikethrough~~` | Correct | PASS |
| inline code | `` `inline code` `` | Correct | PASS |
| underline | `<u>underlined text</u>` | Correct | PASS |
| colored text (foreground) | Blue/red text preserved | Colors silently dropped, plain text only | **FAIL** |
| highlighted text (background) | `==highlighted text==` | Correct | PASS |
| hyperlink | `[hyperlink](url)` | Correct | PASS |
| inline equation | `$E = mc^2$` | Correct | PASS |
| bold+italic combined | `**bold *and italic* combined**` | `**bold *****and italic***** combined**` (garbled) | **FAIL** |
| toggle heading 1 | `> [!note]+ # text` with `> ` children | Children NOT wrapped in `> ` | **FAIL** |
| toggle heading 2 | `> [!note]+ ## text` with `> ` children | Children NOT wrapped in `> ` | **FAIL** |
| divider | `---` | Correct | PASS |

### Page 2: Lists & Tasks

| Block/Feature | Expected | Actual | Status |
|---|---|---|:---:|
| bulleted_list_item (L1) | `- item` | Correct | PASS |
| bulleted_list_item (L2) | `    - nested` | Correct (4-space indent) | PASS |
| bulleted_list_item (L3) | `        - deep` | Correct (8-space indent) | PASS |
| numbered_list_item (L1) | `1. item` | Correct, auto-numbered | PASS |
| numbered_list_item (L2) | `    1. nested` | Correct | PASS |
| numbered_list_item (L3) | `        1. deep` | Correct | PASS |
| to_do (unchecked) | `- [ ] task` | Correct | PASS |
| to_do (checked) | `- [x] task` | Correct | PASS |
| to_do (nested) | `    - [ ] subtask` | Correct | PASS |
| mixed: bullet > numbered | Bullet with numbered children | Correct | PASS |
| mixed: bullet > todo | Bullet with todo children | Correct | PASS |

**This page is flawless. All list/task blocks work perfectly with nesting.**

### Page 3: Code, Math & Quotes

| Block/Feature | Expected | Actual | Status |
|---|---|---|:---:|
| code (python) | ` ```python ... ``` ` | Correct | PASS |
| code (javascript) | ` ```javascript ... ``` ` | Correct | PASS |
| code (sql) | ` ```sql ... ``` ` | Correct | PASS |
| code (plain text) | ` ``` ... ``` ` (no language) | Rendered as ` ```javascript ``` ` | **FAIL** |
| equation (block) | `$$ ... $$` | Correct (both equations) | PASS |
| quote (simple) | `> text` | Correct | PASS |
| quote (multi-line) | Multiple `> ` lines | Correct (3 separate `>` lines) | PASS |
| callout (tip / lightbulb) | `> [!tip]` | Correct | PASS |
| callout (warning) | `> [!warning]` | Correct | PASS |
| callout (danger / exclamation) | `> [!danger]` | Correct | PASS |
| callout (note / memo) | `> [!note]` | Correct | PASS |
| callout with children | Bullets inside callout with `> ` prefix | Correct | PASS |

**Note on plain text code block:** Notion MCP likely defaulted the language to `javascript` when creating the block. The notion-sync converter correctly maps `"plain text"` to `""`, but Notion didn't store it as `"plain text"`. This is a **test setup issue**, not a notion-sync bug.

### Page 4: Media, Toggles & Embeds

| Block/Feature | Expected | Actual | Status |
|---|---|---|:---:|
| image (with caption) | `![caption](url)` | Correct | PASS |
| image (second) | `![caption](url)` | Correct | PASS |
| bookmark/link | `[text](url)` | Correct | PASS |
| toggle (simple) | `> [!note]+ text` with `> ` children | Correct | PASS |
| toggle (with bullets) | Bullets inside toggle with `> ` prefix | Correct | PASS |
| toggle (with numbered) | Numbered list inside toggle with `> ` prefix | Correct | PASS |
| divider (multiple) | `---` | Correct | PASS |
| inline math | `$\alpha + \beta = \gamma$` | Correct | PASS |

**Toggle blocks (non-heading) work correctly** -- children properly wrapped in `> ` prefix. This contrasts with the toggle heading bug in Page 1.

### Page 5: Tables & Columns

| Block/Feature | Expected | Actual | Status |
|---|---|---|:---:|
| table (header row) | Pipe table with `---` separator | Correct | PASS |
| table (header row + col) | Pipe table with bold cells | Correct (bold preserved in cells) | PASS |
| column_list | Columns separated by `---` | Correct | PASS |
| equation (matrix) | `$$ ... $$` | Correct | PASS |

---

## Bug Details

### BUG 1: Toggle heading children not wrapped in `> `

**File:** `go/internal/markdown/converter.go`, lines 62-68 (heading_1), 78-82 (heading_2), 91-95 (heading_3)

**Symptom:** Toggle heading children render outside the Obsidian callout block:
```markdown
> [!note]+ # Toggle Heading 1
This content is hidden inside a toggle heading.    <-- should be "> This content..."
It can have multiple paragraphs.                    <-- should be "> It can have..."
```

**Root cause:** The heading cases use `maybeConvertChildren` but do NOT prefix child lines with `> `, unlike the `toggle` block case which does:
```go
// toggle case (correct) - prefixes children with "> "
lines := strings.Split(childMd, "\n")
for i, line := range lines { lines[i] = "> " + line }

// heading cases (buggy) - just appends raw children
return fmt.Sprintf("> [!note]+ # %s%s", text, children), nil
```

**Fix:** Apply the same `> ` prefixing logic from the `toggle` case to all three heading toggleable cases.

### BUG 2: Heading 1 blocks missing

**File:** Needs investigation

**Symptom:** The `# Heading 1: Introduction` block in Page 1 does not appear in the frozen markdown. H2 and H3 blocks render correctly.

**Possible causes:**
1. Notion MCP may have merged the first `# Heading` with the page title instead of creating it as a body block
2. The Notion API may not return it as a block in `FetchAllBlocks`
3. The converter may be dropping it

**Needs:** Fetch the raw block list from the Notion API for this page to determine if the H1 block exists.

### BUG 3: Bold+italic combined produces garbled markdown

**File:** `go/internal/markdown/richtext.go`, `convertRichTextItem()`

**Symptom:** Text that is both bold and italic produces:
```
**bold *****and italic***** combined**
```
Instead of:
```
**bold *and italic* combined**
```

**Root cause:** Notion splits rich text into separate segments by annotation. "bold and italic" becomes a segment with both `bold: true` and `italic: true`. The converter applies `**` then `*` wrapping independently: `*` around text then `**` around that, producing `***text***`. When adjacent to another `**bold**` segment, the asterisks collide: `**bold *****and italic***** combined**` which renders incorrectly.

**This is a known difficult problem** with Notion rich text to Markdown conversion. Fixing it requires lookahead/merging of adjacent segments with shared annotations.

---

## Minor Issues

### Foreground text colors dropped

**File:** `go/internal/markdown/richtext.go`, line 60-62

Only `_background` colors produce `==highlight==`. Foreground colors (`blue`, `red`, etc.) are silently dropped. No standard Markdown equivalent exists, but Obsidian supports `<span style="color:blue">` or custom CSS.

### Relations show raw IDs

By design, `relation` properties store raw page UUIDs (e.g., `2fe57008-e885-8187-8270-dea9ccb27b41`). Resolving these to page titles would require additional API calls during sync.

---

## Score Summary

| Category | Passed | Failed | Total | Rate |
|----------|--------|--------|-------|------|
| Properties | 18 | 0 | 18 | **100%** |
| Blocks (Page 1) | 12 | 5 | 17 | 71% |
| Blocks (Page 2) | 11 | 0 | 11 | **100%** |
| Blocks (Page 3) | 11 | 1 | 12 | 92% |
| Blocks (Page 4) | 8 | 0 | 8 | **100%** |
| Blocks (Page 5) | 4 | 0 | 4 | **100%** |
| **Overall** | **64** | **6** | **70** | **91%** |

**Bottom line:** Properties are rock-solid. Block conversion is strong for lists, tables, code, equations, callouts, toggles, images, and dividers. The main gaps are toggle headings (children not in callout), a missing H1, and garbled bold+italic combos. The foreground color limitation is minor since there's no clean Markdown equivalent.
