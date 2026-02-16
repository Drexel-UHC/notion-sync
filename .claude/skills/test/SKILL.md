---
name: test
description: Run all tests — unit tests, then system integration tests against real Notion databases
version: 1.0.0
args: "[--verbose] [--no-cleanup] [--skip-unit] [--skip-system]"
---

# Unified Test Runner

Run all test suites sequentially. API rate limits prevent concurrent Notion calls, so system tests run one at a time.

## Flags

Parse the skill args for these flags:

| Flag | Effect |
|------|--------|
| `--verbose` | Pass through to system test skills |
| `--no-cleanup` | Pass through to system test skills |
| `--skip-unit` | Skip Step 1 (Go unit tests) |
| `--skip-system` | Skip Steps 2 and 3 (system integration tests) |

## Steps

### Step 1: Unit Tests

**Skip if `--skip-unit` was passed.**

Run: `go test ./... -count=1`

- **Pass criteria:** exit code 0, all tests pass.
- Print summary: `Step 1: Unit tests — X packages, Y tests, PASS/FAIL`

### Step 2: Single Data Source System Test

**Skip if `--skip-system` was passed.**

Invoke the `/test-single-datasource-db` skill, passing through `--verbose` and `--no-cleanup` flags if present.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print summary: `Step 2: Single datasource — PASS/FAIL`

### Step 3: Double Data Source System Test

**Skip if `--skip-system` was passed.**

Invoke the `/test-double-datasource-db` skill, passing through `--verbose` and `--no-cleanup` flags if present.

- **Pass criteria:** Skill reports `ALL TESTS PASSED`.
- Print summary: `Step 3: Double datasource — PASS/FAIL`

## Summary

Print a combined results table:

```
| Suite                    | Result |
|--------------------------|--------|
| Unit tests (go test)     | PASS   |
| Single datasource system | PASS   |
| Double datasource system | PASS   |
```

Then:
- If all suites passed: `✅ ALL SUITES PASSED`
- If any failed: `❌ SOME SUITES FAILED` followed by which ones failed
