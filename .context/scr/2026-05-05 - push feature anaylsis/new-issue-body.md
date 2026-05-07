# Epic: Push feature — design re-evaluation

**Status:** epic
**Date:** 2026-05-05
**Trigger:** issue #55 (rich_text/title flattening) surfaced a deeper design gap — push was added without an explicit conflict / source-of-truth model, and operates at row granularity when the API supports cell granularity.

This epic covers three things:

1. **Document how push works today.** A flow diagram of end-to-end push behavior so the design is legible before we change it.
2. **Make push only send fields the user actually changed.** Today push re-sends every editable field on every row, even untouched ones. Diff each field against Notion's current value and only send genuine local edits.
3. **Verify rich-text/title formatting survives.** The original #55 symptom — bold/links on unedited fields get flattened on push — used as the acceptance test that #2 actually fixed the problem.

Why it matters: today's "send everything" shortcut bumps `last_edited_time` on every push (firing automations, polluting edit history), strips formatting on fields nobody touched, and burns API quota on no-ops. All three fall out of one architectural shape — push doesn't diff before sending. Fix the diff, fix all three.
