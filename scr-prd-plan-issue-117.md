# Plan — Drexel-UHC/notion-sync#117: [Push] Create new rows in an existing database (properties only)

**PR title:** feat(push): create new rows from `notion-create: true` files

> Triage scorecard: impact 6 · difficulty 4 · blast radius 4 · risk 5 · testability 8
> Verdict at review: FIX THE ISSUE FIRST — real use case, modest work; as written it would turn any stray `.md` into a row and omits the duplicate-row guard. This plan fixes both. Issue: https://github.com/Drexel-UHC/notion-sync/issues/117

## Problem

`push` (v2.0.0) only writes property changes for rows that already exist in Notion. A local `.md` without `notion-id` has no path into Notion — today the classifier (`internal/sync/validation.go`, `ClassHaltUnexpected`) halts the whole run on it as a stray. The OBT loader (SALURBAL-Climate/salurbal-obt#58) needs to add one row per dataset version to the DMC Opt-in database, properties only, and currently has to do that with a separate Notion write outside notion-sync.

Two corrections to the issue's framing, agreed at review:

1. **Opt-in is explicit.** A file without `notion-id` is still a stray and still halts. Only a file carrying the frontmatter marker `notion-create: true` is a create candidate. Intent lives with the file, not the invocation — no CLI flag; the loader just writes the marker.
2. **Duplicate rows must be impossible without a human step.** Notion has no idempotency key and no client-chosen page ID. If `POST /pages` succeeds but the local write-back fails, a naive re-run would create the row again. The guard is a two-phase local stamp: the marker flips to `notion-create: pending` *before* the POST; a `pending` file halts every later run. Recovery reuses what already exists — `notion-sync refresh` imports the created row as `{id}.md`, and the user deletes the pending file.

## Approach

Everything hangs off the existing classifier → gate → loop → write-back → summary pipeline in `internal/sync/`. Creates are a new class that flows through the same preview and gate, then dispatch to their own function in the loop. The update path is untouched.

**Marker lifecycle** (frontmatter key `notion-create`):

| State | Meaning | Classifier result |
|---|---|---|
| `notion-create: true`, no `notion-id` | create candidate | `ClassReadyCreate` (queued) |
| `notion-create: pending` | POST may have fired; write-back not confirmed | `ClassHaltCreatePending` (halts run) |
| absent, no `notion-id` | stray | `ClassHaltUnexpected` (unchanged) |
| `notion-create: true` **and** `notion-id` set | contradictory | `ClassHaltMalformed`-style halt with reason |
| absent after a successful create | normal row | `ClassReady` |

**Per-create sequence** (new `createRow` function in `internal/sync/push.go`, called from the `PushDatabase` loop when `f.Class == ClassReadyCreate`):

1. Build the payload with `buildPropertyPayloadFor(f.fm, schema, nil)` — nil `only` → every pushable field present in frontmatter. Validation errors → `FailedRows`, same as updates.
2. Require the schema's title property (the one key in `schema` with `Type == "title"`) to be present and non-empty in frontmatter; missing → per-row failure with fix "add `<TitleProp>:` to the frontmatter". (Default chosen at authoring; Notion would accept an untitled page, but an untitled row in the Opt-in DB is never what the loader means.)
3. If `opts.DryRun` → count as created, record `PushedEntry{File, Fields}` under `CreatedRows`, **no file write, no stamp**.
4. Stamp: rewrite the marker line in the file from `notion-create: true` → `notion-create: pending` (string-anchored edit, same style as `updateAfterPush`). Stamp failure → per-row failure, no POST.
5. `opts.Client.CreatePage(metadata.DataSourceID, properties)` → `POST /pages` with `parent: {"type": "data_source_id", "data_source_id": ...}`. Legacy metadata with no `DataSourceID` → use `parent: {"database_id": ...}`. Auth error → `AuthHalted` break, same as updates. Other error → per-row failure; **the stamp stays `pending`** (the POST may have partially succeeded on Notion's side; the next run halts and the human reconciles via refresh).
6. Write-back (`finalizeCreate`): remove the `notion-create` line; insert `notion-id`, `notion-url` (canonicalized via `notion.CanonicalizeNotionURL`), `notion-last-edited`, `notion-database-id`, `notion-last-pushed` at the top of the frontmatter in the same key order `FreezePage` uses. Write-back failure → per-row failure; stamp stays `pending` (guard engaged).
7. **No rename in push.** The file keeps its local name; the existing `migrateToUUIDFilenames` on the next `refresh` renames it to `{id}.md`, exactly as it does for legacy title-named files.

**Gate / preview:** `BuildPushQueue` runs `classifyFolder` network-free and already separates `Queue` from `LocalHalts`. Add `Creates []string` to `PushPreview`; `ClassHaltCreatePending` is a local halt (no network needed) so it joins `LocalHalts` and the user sees it at consent time. The CLI preview prints creates as their own block ("will CREATE 2 new rows:").

**Conflict / TOCTOU:** not applicable to creates — nothing to re-fetch. `--force` changes nothing for creates (the schema is still fetched under `--force`; only the gate's network checks are skipped).

**Summary:** `PushResult` gains `Created int` and `CreatedRows []PushedEntry`; `RunSummary` gains `"created": [...]` (same `PushedEntry` shape). `status()` treats creates like pushes for clean/partial. Human renderer prints a `Created: N` line.

**Client:** `CreatePage(parentDataSourceID string, properties map[string]interface{}) (*notion.Page, error)` on `internal/notion/client.go`, the `sync.NotionClient` interface in `internal/sync/client.go`, and `mockNotionClient` in `internal/sync/mock_client_test.go` (record `createRequests`, assign a generated ID, support `createErrors` per title for failure injection).

**Docs:** `internal/sync/agents.md.tmpl` — new "Creating rows" subsection under push: the marker, the pending halt and its fix, the no-body rule, the post-create key set, the rename-on-refresh. `CLAUDE.md` push notes if any exist.

## Open decisions

none

## Agreed Moves

- [ ] Move 1 — Add a create-row call to the Notion client and test mock
- [ ] Move 2 — Classify opted-in files without an ID as new rows; other strays still halt
- [ ] Move 3 — Create rows with the pending stamp, then save the ID back and rename the file
- [ ] Move 4 — Show new rows in the preview and run summary
- [ ] Move 5 — Unit tests, a `/test-push` case, and an AGENTS.md update

> Note on Move 3's "rename the file": at authoring, the agreed simplification stands — push does not rename; the existing `migrateToUUIDFilenames` on the next `refresh` does. The file still ends up as `{id}.md`; the step just lives where it already exists.

## Implementation steps

1. **(Move 1)** `internal/notion/client.go`: add `CreatePage(dataSourceID string, properties map[string]interface{}) (*Page, error)` → `POST /pages` with `parent.data_source_id`; fall back to `parent.database_id` when the caller passes an empty data source ID and a database ID (second param or a small options struct — keep the interface to one method). Unit test the request body shape with an `httptest` server if `client_test.go` already has that harness; otherwise cover via the mock.
2. **(Move 1)** `internal/sync/client.go`: add `CreatePage` to the `NotionClient` interface. `internal/sync/mock_client_test.go`: implement it — record the call, mint `created-<n>` IDs, return a `*notion.Page` with `ID`, `URL`, `LastEditedTime`, and the echoed properties; add `createErrors map[string]error` keyed by title for failure injection. Compile passes.
3. **(Move 2, TDD — red first)** Write step 8 cases (b), (c), (g), (i) first; run, confirm each fails for the right reason, then implement below until green. `internal/sync/validation.go`: add `ClassReadyCreate` and `ClassHaltCreatePending` to the class enum; `ClassHaltCreatePending.isLocalHalt()` and `.IsHalt()` true. In `classifyFolder`, before the `!hasID` stray check: read `fm["notion-create"]`; `pending` → halt with reason "a previous push may have created this row in Notion — run `notion-sync refresh` to pull it, then delete this file"; `true` with `notion-id` set → halt "notion-create: true on a file that already has a notion-id — remove one"; `true` without id → `ClassReadyCreate` (skip option validation? **no** — run `validateRowOptions` on creates too; an unknown select value would auto-create a junk option exactly as on updates). Every other path unchanged.
4. **(Move 2)** `internal/sync/push.go` `readyQueue`: include `ClassReadyCreate`. `internal/sync/summary.go` `fixFor`-style switch: add the pending-halt fix string.
5. **(Move 3, TDD — red first)** Write step 8 cases (a), (d), (e), (f), (h) first — assert file state and mock calls only (summary fields land in Move 4); confirm red, then implement until green. `internal/sync/push.go`: add `stampCreatePending(path)`, `finalizeCreate(path, page, dbID, pushedAt)` (frontmatter string edits, CRLF-normalized like `updateAfterPush`), `titlePropertyKey(schema)`, and `createRow(opts, metadata, schema, f, result)` implementing the per-create sequence in Approach. In the `PushDatabase` loop, dispatch `ClassReadyCreate` → `createRow` before the update-only diff/TOCTOU block.
6. **(Move 4)** `internal/sync/types.go`: `PushPreview.Creates []string`; `PushResult.Created int`, `CreatedRows []PushedEntry`. `BuildPushQueue` populates `Creates`. `internal/sync/summary.go`: `RunSummary.Created []PushedEntry` (`json:"created"`, always non-nil → `[]`), `status()` counts creates as writes.
7. **(Move 4)** `cmd/notion-sync/main.go` `runPush`: preview prints a "will CREATE N new rows" block from `preview.Creates` after the update queue; dry-run and real-run human summaries print `Created: N`; the confirmation prompt's count includes creates.
8. **(Move 5)** `internal/sync/push_test.go`: cases (a)–(i) already exist from the TDD steps 3 and 5; here add (j) and extend (a)/(d)/(f) with `Created`/`CreatedRows` assertions now that Move 4 landed. Full case list — (a) marker file → created, write-back keys present, marker removed, mock saw one POST with title + pushable props; (b) stray without marker still `ClassHaltUnexpected`; (c) `pending` file halts the run before any write; (d) POST error leaves `pending` stamp, run continues, summary `failed` names it; (e) write-back error (inject via read-only file or a test hook) leaves `pending`; (f) dry-run creates nothing, no stamp; (g) marker + `notion-id` halts; (h) missing title property → per-row failure; (i) unknown select value on a create → `ClassHaltInvalidOption`; (j) `BuildPushQueue` lists the create and the pending halt. `summary_test.go`: `created` array shape and `[]` when empty.
9. **(Move 5)** `.claude/skills/test-push/SKILL.md`: add Phase 5 — "Create rows": write `new-row.md` with `notion-create: true` and the fixture's title/select props; `push --dry-run` shows it under created with no Notion write; `push --yes` creates it; assert file now has `notion-id`/`notion-url`/`notion-last-edited`/`notion-database-id`/`notion-last-pushed` and no `notion-create`; `refresh` renames it to `{id}.md`; cleanup archives the created page via the API so the fixture DB stays re-runnable (follow the skill's existing state-invariants section).
10. **(Move 5)** `internal/sync/agents.md.tmpl`: "Creating rows" subsection (marker, pending halt + fix, properties-only/no body, post-create keys, rename on refresh, `created` in the run-summary JSON). No manual version bump — the stamp is the binary's `Version`, interpolated at render. `internal/sync/agents_test.go`: assert the new section is present.
11. `go build ./cmd/notion-sync && go test ./...` green; delete this plan file in the shipping PR.

## Success spec

| Criterion | Check | Phase |
|---|---|---|
| Marker file creates one row; write-back keys present; marker removed | `go test ./internal/sync/ -run TestPush.*Create` | agent |
| File without marker and without `notion-id` still halts as stray | `go test ./internal/sync/ -run TestPush.*Stray` | agent |
| `notion-create: pending` halts the run before any write | `go test ./internal/sync/ -run TestPush.*Pending` | agent |
| POST or write-back failure leaves `pending`; re-run halts, never re-creates | `go test ./internal/sync/ -run TestPush.*Pending` | agent |
| `--dry-run` reports the create and writes nothing (no POST, no stamp) | `go test ./internal/sync/ -run TestPush.*DryRun.*Create` | agent |
| Run-summary JSON carries `created: []` on every terminal path | `go test ./internal/sync/ -run TestSummary` | agent |
| Unknown select value on a create halts with `ClassHaltInvalidOption` | `go test ./internal/sync/ -run TestPush.*Create.*Option` | agent |
| Whole suite green | `go test ./...` | agent |
| Real Notion: `push --yes` on a marker file creates the row; preview and summary show it; `refresh` renames to `{id}.md` | `/test-push` Phase 5 | human-pre-merge |
| Downstream AGENTS.md documents the marker and the pending recovery after a `refresh` | build with `-ldflags "-X main.version=<new>"` (dev build skips AGENTS.md), refresh any synced folder, open `./notion/AGENTS.md` | human-pre-merge |
| OBT loader (salurbal-obt#58) can add a dataset-version row end to end with notion-sync alone | run the loader against the DMC Opt-in DB on the released binary | human-post-merge |

## Out of scope

- Page body push — body text in a marker file is ignored, not sent, not warned about (a warning is a follow-up if it ever bites).
- Creating databases or data sources.
- Renaming the created file inside push — `refresh` already does it via `migrateToUUIDFilenames`.
- Automatic reconciliation of a `pending` file (title-match query, created-after timestamp lookups). The guard halts; the human runs `refresh` and deletes the file.
- A CLI flag for creates — the marker is the only opt-in.
- Changing any existing-row update behaviour, gate ordering, or the `--force` escape hatch.
