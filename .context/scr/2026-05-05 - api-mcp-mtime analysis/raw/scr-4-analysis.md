# scr-4 gap analysis — what was found, what to do next

Companion to `scr-4.md`. Empirical test of: "do property writes bump `last_edited_time`?"

## Headline finding

**scr-4.md's premise does not reproduce.** On test database `2fe57008-e885-8003-b1f3-cc05981dc6b0`, every property type bumped `last_edited_time` when written via the public Notion REST API — including the case scr-4.md flagged (`url → null`).

A spot-check on the MCP path (rich_text write via `mcp__claude_ai_Notion__notion-update-page`) also bumped mtime.

So the "agent writes silently skipped by refresh" failure mode described in scr-4.md was almost certainly **not** caused by Notion not bumping mtime on URL clears. Real cause is still unknown — re-investigate PR #580 with that assumption removed.

## Results — API path (all 15 measured)

| property         | API bumps mtime? |
| ---------------- | ---------------- |
| title            | Yes              |
| rich_text        | Yes              |
| number           | Yes              |
| select           | Yes              |
| multi_select     | Yes              |
| date             | Yes              |
| checkbox         | Yes              |
| url (set)        | Yes              |
| url (clear→null) | Yes              |
| email            | Yes              |
| phone_number     | Yes              |
| relation (set)   | Yes              |
| relation (clear) | Yes              |
| files (set)      | Yes              |
| files (clear)    | Yes              |

## Results — MCP path

Only `rich_text` was measured to completion (bumped=Yes). The other 13 MCP writes succeeded but their after-timestamps were not read. Given API behavior is uniform and rich_text MCP matches, no reason to expect MCP path differs.

## Other PR #580 hypotheses worth investigating

- Refresh ran *before* the API write committed (eventual consistency window)
- Page ID mismatch / wrong data source
- The clear was done via Notion UI, not API, and was reverted
- `notion-last-edited` in the local frontmatter was wrong/stale, not Notion's `last_edited_time`
- Database had a property-level rule that suppressed mtime bump (unlikely, but check)

## ⚠️ For the next agent: KEEP THE TEST CHEAP

The first test took ~30 minutes, mostly because:

1. **`last_edited_time` is quantized to whole minutes** (always `:HH:MM:00.000Z`). Probes faster than a minute apart give false negatives. The test must straddle minute boundaries — that's the irreducible cost.
2. **15 properties × straddle = bloat.** You don't need 15. Test 3: `rich_text`, `url-clear`, `checkbox`. If they all bump, you're done. Add others only if one fails.
3. **One page, sequential writes is wrong.** Within one minute, all writes look like one bump. Use one fresh page per property type written in parallel, OR a single page with one minute between writes.
4. **Don't test files-set via MCP.** Needs an uploaded file ID. Skip.

### Minimum viable rerun (~3 minutes total)

```
1. Create 3 fresh pages on the test database (one for each test prop).
2. Wait into next minute (M1).
3. Read baseline last_edited_time on all 3 (will be M0).
4. Issue ONE property write per page (API or MCP — pick one path, or both via 6 pages).
5. Wait into next minute (M2).
6. Read after-timestamp on all 3.
7. bumped = after > baseline.
```

API key lives in Windows Credential Manager target `notion-sync:api-key`, stored as UTF-8 bytes (NOT UTF-16). Read in PowerShell via `CredRead` PInvoke, decode as UTF-8.

## Test artifacts

- `scr-4-test-api.py` — full 15-property API test (parallel pages, minute-aware)
- `scr-4-mcp-setup.py` / `scr-4-mcp-finalize.py` — two-phase MCP test (split because MCP writes are tool calls between Python invocations)
- `scr-4-results-api.json` — API results
- Test database: `2fe57008-e885-8003-b1f3-cc05981dc6b0` ("test database obsdiain complex")
- All test pages were archived after the run.
