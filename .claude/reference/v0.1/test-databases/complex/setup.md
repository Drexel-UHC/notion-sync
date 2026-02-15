# Test Database: Complex (Property & Block Coverage)

Reproducible protocol for creating a comprehensive Notion test database that exercises all supported property types and block types in `notion-sync`.

---

## Prerequisites

- Notion MCP tools available (search, fetch, create-pages, update-page, update-data-source)
- A parent page in Notion to host the database (or create one)
- Familiarity with `notion://docs/enhanced-markdown-spec` (fetch via MCP resource before writing content)

---

## Step 1: Create or Locate the Database

Create an empty database under the desired parent page, or receive an existing empty database ID from the user.

```
Database name: test database obsidian - complex
```

## Step 2: Add Property Columns

Use `update-data-source` to add all supported property types to the database schema in a single call.

### Property Schema

| Property Name  | Type              | Config                                                                 |
|----------------|-------------------|------------------------------------------------------------------------|
| Name           | title             | (already exists by default)                                            |
| Description    | rich_text         | `{}`                                                                   |
| Score          | number            | `{"format": "number"}`                                                 |
| Category       | select            | Options: Research (blue), Engineering (green), Design (purple), Marketing (orange) |
| Tags           | multi_select      | Options: urgent (red), frontend (blue), backend (green), docs (gray), bug (pink) |
| Due Date       | date              | `{}`                                                                   |
| Approved       | checkbox          | `{}`                                                                   |
| Website        | url               | `{}`                                                                   |
| Contact Email  | email             | `{}`                                                                   |
| Phone          | phone_number      | `{}`                                                                   |
| Assignee       | people            | `{}`                                                                   |
| Attachments    | files             | `{}`                                                                   |
| Related        | relation          | Self-relation: `{"data_source_id": "<this db's data_source_id>", "type": "single_property", "single_property": {}}` |
| Created        | created_time      | `{}`                                                                   |
| Last Edited    | last_edited_time  | `{}`                                                                   |
| Created By     | created_by        | `{}`                                                                   |
| Last Edited By | last_edited_by    | `{}`                                                                   |
| ID             | unique_id         | `{"prefix": "TASK"}`                                                   |

### Known Limitations

- **status**: Cannot be created via API `update-data-source`. Must be added manually in Notion UI if needed.
- **people/assignee**: Can be added as a column but setting values requires valid user IDs.
- **files/attachments**: Column can be created but uploading files via API is not straightforward.

---

## Step 3: Create Pages (5 rows)

Use `create-pages` with `parent: {"data_source_id": "<data_source_id>"}`. All 5 pages created in a single API call.

### Page 1: "Headings & Rich Text"

**Purpose:** Test heading levels, toggle headings, and all inline formatting.

**Properties:**
```
Name: "Headings & Rich Text"
Description: "Tests all heading levels and inline formatting"
Score: 95
Category: "Research"
Tags: ["urgent", "docs"]
Due Date: 2026-03-15 (date only)
Approved: true
Website: "https://example.com/research"
Contact Email: "alice@example.com"
Phone: "+1-555-0101"
```

**Block types exercised:**
- `heading_1`, `heading_2`, `heading_3`
- `paragraph` with: **bold**, *italic*, ~~strikethrough~~, `inline code`, underline, colored text, highlighted text, [hyperlink], inline equation ($E = mc^2$)
- Toggle heading 1 (`heading_1` with `is_toggleable: true`)
- Toggle heading 2 (`heading_2` with `is_toggleable: true`)
- `divider`

### Page 2: "Lists & Tasks"

**Purpose:** Test all list types with nesting.

**Properties:**
```
Name: "Lists & Tasks"
Description: "Tests bulleted lists, numbered lists, and to-do items with nesting"
Score: 42
Category: "Engineering"
Tags: ["frontend", "backend"]
Due Date: 2026-02-01 -> 2026-04-30 (date range)
Approved: false
Website: "https://example.com/engineering"
Contact Email: "bob@example.com"
Phone: "+44-20-7946-0958"
```

**Block types exercised:**
- `bulleted_list_item` (3 nesting levels)
- `numbered_list_item` (3 nesting levels)
- `to_do` (checked and unchecked, with nesting)
- Mixed: bullets with numbered children, bullets with todo children

### Page 3: "Code, Math & Quotes"

**Purpose:** Test code blocks, equations, quotes, and callouts.

**Properties:**
```
Name: "Code, Math & Quotes"
Description: "Tests code blocks, equations, quotes, and callouts"
Score: 78.5
Category: "Design"
Tags: ["docs"]
Due Date: 2026-01-10 (date only)
Approved: true
Website: "https://example.com/design"
Contact Email: "carol@example.com"
Phone: "+81-3-1234-5678"
```

**Block types exercised:**
- `code` blocks: python, javascript, sql, plain text (no language)
- `equation` (block-level, multiple)
- `quote` (simple and multi-line via `<br>`)
- `callout` with icons: light bulb (tip), warning (warning), exclamation (danger), memo (note)
- Callout with child blocks (bulleted list inside callout)

### Page 4: "Media, Toggles & Embeds"

**Purpose:** Test media blocks, toggles, and embeds.

**Properties:**
```
Name: "Media, Toggles & Embeds"
Description: "Tests images, bookmarks, embeds, toggles, and dividers"
Score: 0
Category: "Marketing"
Tags: ["urgent", "bug"]
Due Date: (empty/null)
Approved: false
Website: (empty/null)
Contact Email: (empty/null)
Phone: "+1-555-0199"
```

**Block types exercised:**
- `image` (external URLs with captions)
- `bookmark` / links
- `toggle` blocks with children (text, bullets, numbered lists)
- `divider` (multiple)
- Inline math in paragraphs

**Note:** Some properties intentionally left empty/null to test null handling.

### Page 5: "Tables & Columns"

**Purpose:** Test tables, column layouts, and structural blocks.

**Properties:**
```
Name: "Tables & Columns"
Description: "Tests tables, column layouts, and structural blocks"
Score: 100
Category: "Research"
Tags: ["frontend", "backend", "docs"]
Due Date: 2026-06-15T09:00:00Z (datetime, not just date)
Approved: true
Website: "https://example.com/tables"
Contact Email: "eve@example.com"
Phone: "+49-30-1234567"
```

**Block types exercised:**
- `table` with header row (3 columns, 4 rows)
- `table` with header row AND header column (4 columns, 3 rows, formatted cells)
- `column_list` / `column` (2-column layout with headings, bullets, numbered lists)
- `equation` (matrix equation)

---

## Step 4: Wire Up Relations

After pages are created, use `update-page` to set the `Related` property on some pages:

- Page 1 ("Headings & Rich Text") -> relates to Page 2, Page 3
- Page 5 ("Tables & Columns") -> relates to Page 4

This tests the `relation` property type producing arrays of page IDs in frontmatter.

---

## Property Coverage Matrix

| Property Type     | Populated | Null/Empty | Range    |
|-------------------|-----------|------------|----------|
| title             | All 5     | -          | -        |
| rich_text         | All 5     | -          | -        |
| number            | All 5     | -          | 0, 42, 78.5, 95, 100 |
| select            | All 5     | -          | 4 options used |
| multi_select      | All 5     | -          | 1-3 tags each |
| date              | 4 of 5    | 1 null     | date, date range, datetime |
| checkbox          | All 5     | -          | true and false |
| url               | 4 of 5    | 1 null     | -        |
| email             | 4 of 5    | 1 null     | -        |
| phone_number      | All 5     | -          | US, UK, JP, DE formats |
| relation          | 2 of 5    | 3 empty    | 1-2 relations |
| people            | 0 of 5    | All empty  | (needs user IDs) |
| files             | 0 of 5    | All empty  | (needs file upload) |
| created_time      | All 5     | -          | Auto-set |
| last_edited_time  | All 5     | -          | Auto-set |
| created_by        | All 5     | -          | Auto-set |
| last_edited_by    | All 5     | -          | Auto-set |
| unique_id         | All 5     | -          | Auto-set, TASK-1 through TASK-5 |

---

## Block Coverage Matrix

| Block Type            | Page(s) | Nesting Tested |
|-----------------------|---------|----------------|
| paragraph             | 1,2,3,4,5 | -           |
| heading_1             | 1       | -              |
| heading_2             | 1,2,3,4,5 | -           |
| heading_3             | 1,5     | -              |
| heading (toggleable)  | 1       | With children  |
| bulleted_list_item    | 2,5     | 3 levels       |
| numbered_list_item    | 2,5     | 3 levels       |
| to_do                 | 2       | 2 levels       |
| code                  | 3       | python, js, sql, plain |
| equation (block)      | 3,5     | -              |
| equation (inline)     | 1,4     | -              |
| quote                 | 3       | With children  |
| callout               | 3       | 4 emoji types, with children |
| toggle                | 4       | With mixed children |
| divider               | 1,4     | -              |
| image                 | 4       | With captions  |
| bookmark/link         | 4       | With captions  |
| table                 | 5       | With/without header col |
| column_list/column    | 5       | 2-column layout |

### Rich Text Annotations Tested

| Annotation     | Page(s) |
|----------------|---------|
| bold           | 1,2,3   |
| italic         | 1,2,3   |
| strikethrough  | 1       |
| inline code    | 1       |
| underline      | 1       |
| colored text   | 1       |
| highlight (bg) | 1       |
| link           | 1,4     |
| inline equation| 1,4     |

---

## Not Covered (Out of Scope)

These block/property types are either unsupported by `notion-sync` or cannot be created via the Notion MCP:

**Properties:** status (can't create via API), formula, rollup, button, verification

**Blocks:** child_page, child_database, link_to_page, synced_block, video, audio, file, pdf, embed (as block), table_of_contents, breadcrumb

To test these, manually add them in the Notion UI.
