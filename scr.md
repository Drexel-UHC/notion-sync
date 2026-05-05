---
title: 'notion-sync: drop filesystem mtime pinning to Notion last_edited_time'
status: proposal
target_repo: https://github.com/Drexel-UHC/notion-sync
discovered_in: salurbal.org PR #580 (2026-05-05)
---

# Bug: notion-sync silently drops content updates from `git status` on same-length edits

## TL;DR

After every write, notion-sync calls `os.utime()` (or equivalent) to set the local file's `mtime` backwards to match Notion's `last_edited_time`. This actively defeats git's stat-cache: when a refresh produces a content change of the same byte length AND the Notion timestamp didn't bump, `git status` reports the file as clean despite the content having changed on disk. Git workflows then commit the unchanged file, and the update is silently lost.

The Notion timestamp is already preserved in the file's YAML frontmatter (`notion-last-edited:`). Duplicating it as filesystem `mtime` adds no functional value and is the sole source of this bug. **Drop the `os.utime()` call.**

## The bug, demonstrated

Repro: take any Notion entry, change a property to a same-length string via the API (e.g. fix a typo `Calors → Carlos`, both 6 chars), then run `notion-sync refresh --ids "<id>"`.

Stat output of the refreshed file, before and after the refresh:

```
                Before refresh                       After refresh
Size:           1770                                 1770              ← unchanged
Modify (mtime): 2026-04-30 14:29:00.000              2026-04-30 14:29:00.000  ← pinned back
Change (ctime): 2026-05-05 10:41:42                  2026-05-05 11:06:31      ← bumped (write happened)
```

`Change` confirms notion-sync did rewrite the file (inode metadata updated). `Modify` is identical to the second between before and after — pinned to Notion's `last_edited_time` of `2026-04-30T18:29:00 UTC`.

`git status` after the refresh: empty. GitHub Desktop: zero changes shown. The corrected content is on disk; git is blind to it.

Side-by-side with Notion's authoritative timestamp inside the file:

```yaml
---
notion-last-edited: "2026-04-30T18:29:00.000Z"   ← matches the pinned mtime exactly
---
```

## Root cause

Git's stat-cache (the mechanism that makes `git status` fast on large repos) skips re-hashing a file when `(size, mtime, inode)` are all unchanged from the cached entry. By pinning `mtime` to a value that does not move forward, notion-sync ensures the stat-cache stays valid even after a content rewrite. The two conditions for silent loss:

1. Notion's `last_edited_time` did not bump between syncs (e.g. URL-property clears via the Notion API do not bump it; some title-property edits also do not, observed empirically).
2. The new content has the same byte length as the old content.

Both conditions hold for any same-length API-driven correction (typo fixes, equal-length renames, certain property toggles). It is not a rare corner — it is the standard shape of "fix a typo and re-sync."

The condition table:

| Notion `last_edited_time` bumped? | Content size changed? | Git sees the rewrite? |
| --------------------------------- | --------------------- | --------------------- |
| Yes                               | Yes                   | yes                   |
| Yes                               | No                    | yes (mtime delta)     |
| No                                | Yes                   | yes (size delta)      |
| **No**                            | **No**                | **no — silent loss**  |

## Proposed fix

Remove the `os.utime()` (or analogous timestamp-pin) call that runs after writing each file. Let the OS set `mtime` naturally to wall-clock-now at write time, like every other file-writing tool on the system.

Suggested replacement comment in code:

> `mtime` is intentionally left at write-time-now so git's stat-cache correctly invalidates on rewrites. Notion's authoritative timestamp is preserved in the file's `notion-last-edited` frontmatter field, which is git-tracked and human-readable.

## Why removing the pin is safe

The justification for the `utime` pin is presumably "preserve Notion's timestamp on disk for audit/reproducibility." That justification does not survive scrutiny:

- **The timestamp is already preserved.** Every entry file carries `notion-last-edited: "<ISO-8601>"` in YAML frontmatter. That field is the durable, content-level, git-tracked, human-readable source of truth for "when did Notion last touch this entry." Filesystem `mtime` is a redundant copy.
- **Filesystem `mtime` is a poor place for authoritative metadata.** It does not survive `cp`, many editors' save-cycles, `rsync` without `-t`, archive extraction, OS-level backups, or simply being read by a stat-aware indexer. Anything that depends on `mtime` for correctness is fragile.
- **"Two refreshes produce byte-identical metadata" is a fake benefit.** Git tracks content, not metadata. Two refreshes already produce byte-identical _commits_ whether or not the working-tree mtimes match. There is no downstream consumer that benefits from matching mtimes that wouldn't be better served by reading `notion-last-edited` from frontmatter.
- **The current behavior produces a silent-loss bug, observed in production** (salurbal.org PR #580: a typo correction `Juan Calors → Juan Carlos` was issued via the Notion API and `notion-sync refresh --ids` was run, but git did not detect the on-disk change; the commit captured the pre-fix state and the typo persisted to a downstream products catalog).

Cost of removing the pin: none observed. Benefit: `git status` becomes trustworthy after every refresh.

## Impact

Anyone using notion-sync with git as the version-control / QC layer (which is the documented use case in `_etl/_notion-sync/CLAUDE.md` of the salurbal.org repo) is exposed. The exposure surface is every same-length edit on a property whose Notion `last_edited_time` does not advance — a non-trivial fraction of routine Notion operations.

The downstream consequence is that automated agents running `notion-sync refresh` followed by `git add -A && git commit` will produce false-clean commits when the trigger conditions hold. Reviewers cannot catch this from the diff alone (there is no diff), only by independently verifying file content against Notion.

## Where the fix likely lives

Best guess based on behavior: a write-helper inside whatever module performs page-to-markdown materialization. Look for `os.utime` or a wrapper that takes a `last_edited_time` parameter alongside a path. Removing the timestamp-pin call (and the parameter, if present) is the entire change.

## Workaround until upstream fix lands

Force-rehash after every refresh:

```bash
notion-sync refresh "<path>" --ids "<id>" --api-key "$NOTION_API_KEY"
# Then one of:
git add -A                           # forces re-hash on stage (may still miss with stat-cache)
# or, more reliably on Windows:
find <path> -type f -exec touch {} + # bumps mtimes so git invalidates the cache
git status                           # now reports the actual changes
```

Critical reviewers of any notion-sync PR should not trust `git status` until this is fixed upstream.
