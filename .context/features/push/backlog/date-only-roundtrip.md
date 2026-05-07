# Date-only frontmatter properties round-trip through push as datetimes

**Status:** backlog (proper fix; pragmatic workaround shipped in #78)
**Tier:** correctness — affects all date-typed properties on every push

## What

Date-only Notion properties (`is_datetime: false`) get promoted to UTC datetimes (`is_datetime: true`) when push reads the frontmatter and forwards the value to Notion's API.

**Symptom on Notion:**
- Before push: `Due Date` shows as `Jun 1, 2026` (date-only).
- After push (any field edit): `Due Date` shows as `Jun 1, 2026, 12:00 AM UTC` (datetime).
- Notion's response: `is_datetime` flips `false` → `true`, `start` becomes `2026-06-01T00:00:00.000Z` instead of `2026-06-01`.

Once promoted, the property is permanently in datetime form on Notion until manually toggled back. This affects **every** date-typed property in **every** pushed row, regardless of whether the user touched it.

## Root cause

`internal/frontmatter/parser.go:38-51` — `normalizeMapValues` formats parsed `time.Time` values with `time.RFC3339`:

```go
case time.Time:
    m[k] = val.Format(time.RFC3339)   // "2006-01-02T15:04:05Z07:00"
```

`yaml.v3` auto-parses any ISO-shaped scalar (including bare `2026-06-01`) to `time.Time` and **discards the original string**. So by the time `normalizeMapValues` runs, the parser cannot tell whether the user wrote `2026-06-01` or `2026-06-01T00:00:00Z` — both arrive as identical `time.Time` values. Reformatting with RFC3339 always emits the time portion, so date-only signal is destroyed at parse time and propagated to every consumer.

`parseDatePayload` in `push.go` then forwards that lossy string straight into the Notion payload, which Notion correctly interprets as a datetime.

## Pragmatic workaround (shipped in #78)

Added `stripMidnightUTC` to `internal/sync/push.go`. It demotes `YYYY-MM-DDT00:00:00Z` (and `+00:00`, `.000Z`, etc.) back to `YYYY-MM-DD` before sending to Notion, but only when:
- Hour, minute, second, and nanosecond are all zero, AND
- Timezone offset is zero.

This is a heuristic: a user who genuinely wrote a UTC midnight datetime gets demoted to date-only. False-positive surface is small (a real "midnight UTC" datetime is unusual), and the workaround keeps the push contract correct for the common case.

Localized to push so the parser's contract doesn't change for other consumers.

## Why this needs a proper fix anyway

The workaround is fragile across three axes:

1. **Lossy for legitimate midnight-UTC datetimes.** A user who legitimately wants `is_datetime: true` at exactly `T00:00:00Z` cannot express it through frontmatter — push will demote it. Niche but real.
2. **Doesn't fix the parser.** Any future code path that reads dates from frontmatter (e.g. comparison logic for cell-level diff in phase 3, validation in phase 2) will hit the same lossy round-trip and need its own workaround.
3. **Phase 3 (cell-level diff) needs precise round-trip.** Cell-level diff compares local frontmatter to Notion's current value to decide what to push. If the comparison normalizes `2026-06-01` and `2026-06-01T00:00:00Z` differently, every date will look "changed" on every push and we lose the whole point of cell-level. The workaround happens to align them today, but only because `stripMidnightUTC` runs on the push side. If diff logic runs *before* `stripMidnightUTC`, drift returns.

## Proper fix

Replace `yaml.Unmarshal` + `normalizeMapValues` with a `yaml.Node`-based parser that walks the AST and preserves the original scalar string for each value. Two main jobs:

- For values tagged `!!timestamp` (RFC3339 datetime), keep the original string.
- For values tagged `!!str` that happen to look ISO-shaped, leave them as strings.

The yaml.v3 docs example is the canonical shape; `internal/frontmatter/parser.go` becomes ~30-40 lines longer but every consumer gets exact round-trip semantics for free.

## Why deferred

Workaround is sufficient for v1.4.0 push correctness. Proper fix touches a shared parser used by import, refresh, push, and validation — wider blast radius, deserves its own PR + test pass. Schedule alongside or after phase 3 (cell-level diff), since phase 3 will surface any remaining round-trip bugs in adjacent property types (e.g. numbers, multi_select strings that look numeric).

## Acceptance

- `internal/frontmatter/parser.go` parses date-only YAML scalars as plain strings (`"2026-06-01"`), not `time.Time`.
- `stripMidnightUTC` in `push.go` is removed (or kept as a defense-in-depth no-op with a deprecation comment).
- New unit test in `frontmatter_test.go`: parsing `Due Date: 2026-06-01` yields the literal string `"2026-06-01"` in the result map; parsing `Due Date: 2026-06-01T00:00:00Z` yields `"2026-06-01T00:00:00Z"`. Both round-trip identically through `Write` + `Parse`.
- `/test-push` F1 still passes against the canonical date-only `Due Date`, with no within-run drift on Notion.
- Other date consumers (refresh stale-detection, push diff) tested for round-trip stability.

## How discovered

`/test-push` iteration 1 (PR #78): F1 caught the drift on the first run; iteration 2 silently passed because the fixture was already corrupted from iteration 1, exposing a separate test-design weakness (within-run-only comparison) which was also fixed in #78. See PR #78 conversation for the full diagnosis.
