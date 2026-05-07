# Exit code 0 for `cancelled` status

**Status:** backlog (v1.4.1 polish)
**Tier:** behavioral spec
**Coupled to:** `cancelled-status.md`

## What

Define the exit code for a cancelled run as `0` (success). Update the rule:

| Status                      | Exit code |
|-----------------------------|-----------|
| `clean`, `cancelled`        | `0`       |
| `partial`, `halted`         | `1`       |

## Why

The user explicitly chose not to push. That is the correct outcome for the command they ran. Exit `0` lets shell pipelines treat `push` as a no-op when declined, without needing to special-case the JSON.

## Why deferred

Only meaningful once `cancelled-status.md` lands. Ship together.

## Acceptance

- Decline at TTY prompt → `$? == 0`.
- Non-interactive without `--yes` → `$? == 0`, but stderr explains how to opt in.
- Any actual failure (`partial`, `halted`) → `$? == 1`, unchanged.
