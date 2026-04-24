---
name: test
description: Run all tests — unit, system, then cleanup
version: 3.0.0
args: "[--verbose] [--no-cleanup] [--skip-unit] [--skip-system]"
---

# Unified Test Runner

Run all test suites sequentially. System tests leave their output on disk so Step 5 can clean up.

## Flags

Parse the skill args for these flags:

| Flag | Effect |
|------|--------|
| `--verbose` | Pass through to system test skills |
| `--no-cleanup` | Skip Step 5 (final cleanup) — leave `test-output/` on disk |
| `--skip-unit` | Skip Step 1 (Go unit tests) |
| `--skip-system` | Skip Steps 2–4 (system tests) |

## Steps

### Step 1: Unit + Integration Tests

**Skip if `--skip-unit` was passed.**

Run: `go test ./... -count=1`

- **Pass criteria:** exit code 0, all tests pass.
- Print: `Step 1: Unit tests — X packages, PASS/FAIL`

### Step 2: Single Data Source System Test

**Skip if `--skip-system` was passed.**

Invoke `/test-single-datasource-db --no-cleanup`, also passing `--verbose` if present.

The `--no-cleanup` is **always** passed here regardless of user flags — we need the files on disk for later steps.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print: `Step 2: Single datasource — PASS/FAIL`

### Step 3: Double Data Source System Test

**Skip if `--skip-system` was passed.**

Invoke `/test-double-datasource-db --no-cleanup`, also passing `--verbose` if present.

Same as Step 2 — `--no-cleanup` always passed to keep files.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print: `Step 3: Double datasource — PASS/FAIL`

### Step 4: Standalone Page System Test

**Skip if `--skip-system` was passed.**

Invoke `/test-standalone-page --no-cleanup`, also passing `--verbose` if present.

Same as Steps 2–3 — `--no-cleanup` always passed to keep files.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print: `Step 4: Standalone page — PASS/FAIL`

### Step 5: Cleanup

**Skip if `--no-cleanup` was passed.** Print `Step 5: Skipped (--no-cleanup)` and leave files.

Otherwise, clean up everything:

1. Delete `test-output/test database obsdiain complex/` (single-source folder)
2. Delete `test-output/test database - double data source/` (double-source folder)
3. Delete `test-output/pages/` (standalone page folder)
4. If `test-output/` is now empty, delete it.

Print: `Step 5: Cleanup — done`

## Summary

Print a combined results table:

```
| Suite                          | Result  |
|--------------------------------|---------|
| Unit tests (go test)           | PASS    |
| Single datasource system       | PASS    |
| Double datasource system       | PASS    |
| Standalone page system         | PASS    |
```

Then:
- If all suites passed: `ALL SUITES PASSED`
- If any failed: `SOME SUITES FAILED` followed by which ones failed
