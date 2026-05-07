# Phase 1 testing gaps identified during /test-push iterations 1-3

**Status:** backlog (low-priority test hardening)
**Tier:** test coverage — closes regression gaps surfaced by the push-roundtrip incident in #78

## What

Two integration tests in `internal/sync/push_test.go` that lock in the **end-to-end** date-roundtrip path (`frontmatter.Parse` → `buildPropertyPayload` → Notion payload), not just the `stripMidnightUTC` helper in isolation.

```go
// frontmatter.Parse currently lossily promotes `2026-06-01` (date-only) to
// an RFC3339 datetime via yaml.v3's auto-time.Time parsing, which is the
// bug `stripMidnightUTC` exists to paper over. This test pins the *integrated*
// behavior: a date-only YAML scalar must end up as a date-only payload to
// Notion, regardless of which layer is doing the work.
//
// If the proper parser fix lands (see date-only-roundtrip.md) this test
// keeps passing — its purpose then becomes a regression marker against
// future re-breaks.
func TestPush_DateOnlyFrontmatter_RoundTripsAsDateOnly(t *testing.T) {
    fm, err := frontmatter.Parse("---\nDue Date: 2026-06-01\n---\n")
    if err != nil { t.Fatal(err) }
    schema := map[string]notion.DatabaseProperty{"Due Date": {Type: "date"}}
    payload, _ := buildPropertyPayload(fm, schema)
    d := payload["Due Date"].(map[string]interface{})["date"].(map[string]interface{})
    if d["start"] != "2026-06-01" {
        t.Errorf("date-only round-trip broke: got %q, want '2026-06-01'", d["start"])
    }
}

// Mirrors the state import writes when Notion already holds a date-only
// property as a datetime (e.g. after a prior corruption). The fix B path
// must still send a date-only payload — otherwise we'd push the corruption
// back instead of repairing it.
func TestPush_DatetimeFrontmatter_AtMidnightUTC_RoundTripsAsDateOnly(t *testing.T) {
    fm, err := frontmatter.Parse(`---` + "\n" + `"Due Date": "2026-06-01T00:00:00.000+00:00"` + "\n" + `---` + "\n")
    if err != nil { t.Fatal(err) }
    schema := map[string]notion.DatabaseProperty{"Due Date": {Type: "date"}}
    payload, _ := buildPropertyPayload(fm, schema)
    d := payload["Due Date"].(map[string]interface{})["date"].(map[string]interface{})
    if d["start"] != "2026-06-01" {
        t.Errorf("midnight-UTC datetime should demote to date-only on send: got %q", d["start"])
    }
}
```

## Why

Existing `TestBuildPropertyValue_Date_*` tests cover `stripMidnightUTC` in isolation — they pass even if `frontmatter.Parse` later changes its lossy behavior, or if the helper is removed alongside a parser fix that doesn't fully restore round-trip. The actual bug observed in `/test-push` iteration 1 lived in the *integration* between those two layers, and only an integration test catches a future re-break of the integration.

This is a defense-in-depth test: it doesn't exercise new code, it proves the seams between `frontmatter.Parse` and `buildPropertyPayload` agree on date semantics.

## Why deferred

- Push-side fix B + the existing isolated unit tests give acceptable coverage for v1.4.0 ship.
- Adding these now means a follow-up commit on this branch; bundling with the proper parser fix (see `date-only-roundtrip.md`) makes more sense — that PR is *also* going to add `frontmatter_test.go` round-trip tests, and the integration tests should land alongside that work to share the same setup.
- Not blocking phase 2 or phase 3 — neither phase touches date-property serialization.

## Acceptance

- Both tests live in `internal/sync/push_test.go` (or a new `push_roundtrip_test.go`).
- Both tests pass against the **current** `stripMidnightUTC`-based fix B.
- Both tests pass against the future yaml.Node-based parser fix (i.e. they don't pin behavior to a specific implementation, only to the contract).
- A regression where someone removes `stripMidnightUTC` without fixing the parser causes `TestPush_DateOnlyFrontmatter_RoundTripsAsDateOnly` to fail.

## How discovered

`/test-push` iteration 1 in PR #78 caught the date-roundtrip bug in production-equivalent usage. Iterations 2 and 3 surfaced that the existing unit tests covered the helper but not the integrated path through `frontmatter.Parse`. See PR #78 conversation and `date-only-roundtrip.md` for the underlying parser bug this gap is adjacent to.
