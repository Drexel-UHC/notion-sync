# Document explicit status rules in run summary

**Status:** backlog (v1.4.1 polish)
**Tier:** documentation

## What

Add an explicit rule table (in DAG header and CLI docs) defining how `status` is computed from `failed[]` and `halted[]`:

| Status      | Condition                                                  |
|-------------|------------------------------------------------------------|
| `clean`     | `failed[]` empty AND `halted[]` empty                      |
| `partial`   | `failed[]` non-empty AND `halted[]` empty                  |
| `halted`    | `halted[]` non-empty (validation halt OR auth halt)        |
| `cancelled` | user declined at `n13` confirm prompt                      |

## Why

Implementers (and downstream agents parsing the JSON) shouldn't have to derive these rules from scattered diagram nodes. One source of truth.

## Why deferred

Pure docs. Rules are derivable from the existing schema. Land alongside `cancelled-status.md` so the table includes all four values.

## Acceptance

- Rule table appears in DAG header comments AND in user-facing docs (CLI `--help` or AGENTS.md push section).
- Test cases cover each status rule transition explicitly.
