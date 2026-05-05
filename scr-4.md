# notion-sync gap: agent-driven edits silently skipped

## The gap

`notion-sync refresh` decides whether to re-fetch a page by comparing the page's Notion `last_edited_time` against the snapshot's `notion-last-edited`. Newer → re-fetch. Same/older → skip. That's the incremental optimization that keeps refresh cheap on large databases.

**The problem:** Notion's API doesn't bump `last_edited_time` for every kind of property write. Empirically observed in this project:

| Operation                                          | Bumps `last_edited_time`?            |
| -------------------------------------------------- | ------------------------------------ |
| Title text edit (e.g., `Name` typo fix)            | Yes                                  |
| Rich-text property edit                            | Yes (assumed — same family as title) |
| URL property cleared (e.g., `English Link → null`) | **No**                               |
| Other property types via API                       | Unknown — not empirically tested     |
| In-app human edits in Notion UI                    | Yes (always)                         |

So when an agent (Claude, automation, MCP) clears a URL or makes a similar non-bumping write, the page silently looks "untouched" to notion-sync. Vanilla refresh skips it. The snapshot drifts from Notion. Nothing surfaces unless a human diffs carefully.

This happened twice on PR #580: Figueroa + Unar Munguía `English Link` clears were missed by `refresh` and required `--ids` to force-fetch.

## Why this gets worse with agents

Same-session edit-then-sync is recoverable: the agent can track touched IDs and pass them to `refresh --ids`. Cross-session is the real hole:

```
Agent A (Mon)  → MCP write to page X       → timestamp doesn't bump → A's session ends
Agent B (Wed)  → notion-sync refresh       → skips page X (timestamp says stale)
                 → page X never lands in snapshot
```

Agent B has no view of what Agent A touched. There's no shared state. As agent edit volume grows, the probability of cross-session drift grows with it.

## Options considered

| #   | Option                                                                                                                                  | Effort                                         | Pros                                                                   | Cons                                                                                  |
| --- | --------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| 1   | **Operational discipline:** every agent runs `--ids` for touched pages at end of session                                                | None                                           | Trivial to adopt                                                       | Only solves same-session. Cross-session still drifts. Easy to forget.                 |
| 2   | **Upstream `--force-since <timestamp>` flag** in notion-sync — re-fetch any page with `last_synced_at < window` regardless of timestamp | Medium (file upstream issue, wait for release) | Cleanest fix. Cross-session safe. Cheap re-fetch (only recent subset). | Depends on maintainer. Not in our hands.                                              |
| 3   | **Periodic `--no-incremental` full refresh** as a safety net (e.g., weekly cron)                                                        | Low                                            | Catches everything                                                     | Expensive on large DBs. Doesn't prevent drift between runs, just bounds it.           |
| 4   | **Persistent `_pending-refresh.json` queue file** per database — agents append touched IDs, next sync drains                            | Medium                                         | Cross-session safe without upstream change                             | Extra failure modes (crash before append, concurrent writes, git churn on every edit) |
| 5   | **Force-bump trick:** every agent write also re-writes a known-bumping property in the same atomic API call                             | Low                                            | Works without upstream change                                          | Ad-hoc per write, easy to miss, ugly                                                  |
| 6   | **Notion webhooks → external queue** (Azure Function, drain on sync)                                                                    | High                                           | Most robust. Captures all edits agent or human.                        | Significant infra. Overkill until volume justifies.                                   |

## My opinion: standardized `agent_log` column (variant of #5, but as a convention)

Add an `agent_log` rich-text property to every notion-sync-managed database. Agents writing to Notion always append to this column in the same atomic `update_properties` call as their real edit (e.g., `"<ISO timestamp> <agent-id>: <short summary>"`).

Why this is the easiest path:

- Rich-text writes **do** bump `last_edited_time` (verified empirically). So pairing any property write with an `agent_log` append guarantees the timestamp bumps, regardless of what the primary property type is.
- Notion's `update_properties` is atomic across multiple properties — the real edit and the log entry land in one API call. No race window.
- No upstream notion-sync change needed. Works with vanilla incremental refresh today.
- No cross-session coordination needed. The mechanism is "every agent edit bumps the timestamp" — sync correctness no longer depends on knowing who touched what.
- Free audit log as a side effect. Useful for debugging, attribution, and noticing when an agent has been chatty on a page.
- Simple convention to enforce: skill definitions and the `mcp__claude_ai_Notion__notion-update-page` wrapper require the `agent_log` field be set on every write. Lint-able.

Tradeoffs honestly:

- Schema change across every database (one rich-text property each — small but real).
- Convention-bound: if an agent forgets the `agent_log` write, the bug recurs silently for that page. Mitigated by wrapping MCP writes in a helper that requires the field.
- The `agent_log` column will accumulate entries over time. Set a convention to keep it short (last N entries, or rolling buffer) so it doesn't bloat.

## Recommended sequence

1. **Now** — add `agent_log` (rich-text) to every database in `_etl/_notion-sync/v1/`. Document the convention in `_etl/_notion-sync/CLAUDE.md`. Update agent skills (`/notion-sync` and any other Notion-writing skill) to require the field.
2. **Also now** — file the upstream `--force-since` issue on `notion-sync`. The `agent_log` convention is local to us; the upstream fix benefits the whole notion-sync ecosystem and doesn't conflict.
3. **Skip the queue file (#4) and webhook (#6) options** unless `agent_log` proves insufficient at higher edit volumes.
4. **Run a periodic `--no-incremental` (option #3) as a belt-and-suspenders safety net** — quarterly is probably enough given `agent_log` should catch the bulk.

## Open questions

- Does Notion's API return the property write order in `update_properties`, or could the rich-text touch land in a way that doesn't bump the timestamp? Need to spot-check.
- Are there any property types we use that _also_ don't bump `last_edited_time` even when paired with a rich-text write? Unlikely (the bump is page-level, not property-level), but worth confirming.
- What's the right format for `agent_log` entries? Suggest: `<ISO timestamp>\t<agent-id>\t<one-line summary>`, newest line on top, capped at 20 entries.
