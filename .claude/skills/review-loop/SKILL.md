---
name: review-loop
description: >
  Iteratively converge a PR to approval: run a critical code review, autonomously
  fix the in-scope findings, run the project's checks, commit and push, then
  re-review with FRESH eyes — repeating until the verdict is Approve, a hard pass
  cap is hit, or a finding needs a human decision. Composes /critical-code-reviewer
  (the reviewer) and /ship (commit + push). Use when the user wants to "review and
  fix" a PR, "auto-review", "run a review loop", or "converge" a PR to approval.
metadata:
  version: "0.2"
---

# Review Loop

An autonomous **review → triage → fix → verify → push → re-review** loop that drives a PR toward an
Approve verdict. It does **not** invent the review or the shipping mechanics — it orchestrates two
existing skills around a convergence loop with explicit guardrails:

- **[`critical-code-reviewer`](../critical-code-reviewer/SKILL.md)** produces each verdict.
- **[`ship`](../ship/SKILL.md)** handles branching / commit / push semantics when needed.

The whole point is to be *trustworthy while unattended*: it fixes the obvious, escalates the
judgment calls, leaves an audit trail (**a review comment on every pass — posted before the loop acts
on the verdict, so no pass can skip it — plus a fixes comment whenever a pass commits**), and never
merges.

## Invocation

`/review-loop [PR-url-or-number]  [max-passes=N]`

- No argument → review the current branch's PR (or the current diff vs `main` if no PR exists yet).
- `max-passes` defaults to **5**.

**This skill commits and pushes to the PR's branch — and is pre-authorized to do so.** Invoking the
skill *is* the grant: commit + push to the PR's branch runs **without a per-run confirmation prompt**.
That standing grant covers commit + push **only**; it is **never** authorization to merge, force-push,
rewrite history, or touch files outside the PR's scope — those remain escalations (see Triage), and the
`Never` rules at the end still bind every run.

## Default mode: foreground, no watchdog

By default the loop runs **unattended with minimal prompts**:

- **Reviews run in the foreground** — a plain blocking `Agent` spawn. Prompts pass through normally, so
  there's no auto-deny risk and **Step 0 (permission preflight) is skipped**.
- **The hang watchdog is off.** If a review subagent ever stalls on an inference hang, stop it manually
  (`/agents` **Running** tab, or **Ctrl+B**) and re-invoke — there's no auto-timeout in this mode.
- **Commit + push are pre-authorized** (see Invocation) — no per-run confirmation.

So a default run only ever stops to ask you on a genuine **escalation** (an
[escape criterion](#escape-criteria)) or halts at the **pass cap** — nothing else.

**Watchdog mode is opt-in:** enable it only when the user asks for it (or passes `--watchdog`). It
backgrounds each review behind a liveness watchdog so a stalled API call can't freeze an unattended
run — at the cost of running **Step 0 (permission preflight)** first and pre-granting the review's
tools (see [Hang watchdog](#hang-watchdog--bound-each-review-spawn)).

## Staging temp files — `.context/.tmp/` only

The loop stages multi-line markdown as files (comment bodies for `gh pr comment --body-file`,
commit messages for `git commit -F`) because inlining markdown in the shell is a quoting minefield
(PowerShell here-strings, `$` interpolation, CRLF). **Where** you stage matters when unattended:

- **Stage in `.context/.tmp/`** (git-ignored, in-repo so a dead run leaves inspectable artifacts)
  and **clean up there** after each successful post/commit (`Remove-Item` / `rm`).
- **Never stage under `.claude/`** — it's a permission-sensitive path; touching it mid-run pops a
  "sensitive file" prompt that silently stalls the unattended loop.
- **Never stage under `.git/`** — it works (unwatched), but nothing belongs there that git itself
  didn't put there.
- Leftover `.context/.tmp/` files from an interrupted run: post/use them if still current, else
  delete — never commit them.

## The verdict

`/critical-code-reviewer` ends every review with a `## Verdict` line — one of `Approve`,
`Request Changes`, or `Needs Discussion`. The loop acts on that line. No extra contract: the only
consumer of the verdict is this same agent, which reads the reviewer's output directly, so there's no
brittle parser to protect against.

`Approve` means "no blocking issues after rigorous review," not "perfect" — there must be no Blocking
or Required findings outstanding.

## The loop

Run **step 0 once** at the start of the invocation *(watchdog mode only — skipped by default; see
[Default mode](#default-mode-foreground-no-watchdog))*; then for each pass `N` starting at 1, run
steps 1–9:

> **Invariant — a review comment every pass, a fixes comment when you commit.** Each pass posts a
> **review comment** (`Code Review (Pass N)`) in step 2, immediately after the cold review and
> *before* the loop acts on the verdict — so no pass (Approve, escalation, or cap) can skip it, and it
> alone is the source of the pass number. A pass that also commits a fix posts a second
> **fixes comment** (`Pass N — Fixes applied`) in step 8. (The bug this prevents: an escalating
> pass 1 that posts nothing, so the pass-2 Approve comment scans zero prior comments and mislabels
> itself "Pass 1.")

### 0. Permission preflight — once, before pass 1 *(watchdog mode only — skipped by default)*

The loop backgrounds the review (see [Hang watchdog](#hang-watchdog--bound-each-review-spawn)), and a
**background subagent auto-denies any tool call that would otherwise prompt** — so if the session
can't grant the review's tools, the backgrounded review degrades **silently**. Catch that up front
with a **capability probe**. Do **not** parse settings files: grants merge across user / project /
local / CLI / enterprise / permission-mode layers, so a file grep gives false negatives.

1. **Probe.** Spawn a throwaway **background** micro-agent (`Agent`, `run_in_background: true`) whose
   only job is: run the read-only commands the real review actually depends on — `git status`, and
   (when the target is a GitHub PR) `gh auth status` **and** `gh pr diff <PR>` (the command the review
   needs most and the one most likely to be ungranted) — then return exactly `OK` if they all ran, or
   `BLOCKED` if any tool call was denied. Nothing else. The probe can exercise the `Bash`(`git`/`gh`)
   family but **not** the review's `Skill` invocation (probing that means running a real skill), so
   `OK` rules out a broad permission lockout — not every per-tool gap.
2. **`OK`** → the session can background and run the review's git/gh reads; proceed to pass 1 with the
   watchdog. (If a backgrounded review still comes back degraded on an un-probed tool — e.g. `Skill` —
   the [watchdog's foreground fallback](#hang-watchdog--bound-each-review-spawn) covers it.)
3. **`BLOCKED`** → a backgrounded review would silently degrade. **Stop and `AskUserQuestion`** with
   three options:
   - **Add the permissions** — grant the full set listed in
     [Permissions this adds](#permissions-this-adds) (the `Bash(stat:*)` / `Bash(date:*)` /
     `Bash(sleep:*)` poll commands and `Read` / `Grep` / `Glob` matter too, not just
     `Monitor` / `Agent` / `Skill` / `TaskStop` / `Bash`(`gh`/`git`)) in `settings.json` /
     `settings.local.json` (or switch to an `auto` / `bypassPermissions` mode), then re-probe.
   - **Run reviews foreground this session** — no backgrounding, so prompts pass through and the review
     is never degraded, **but the [hang watchdog can't fire](#hang-watchdog--bound-each-review-spawn)**;
     a hang then needs a manual stop (`/agents` → **Running**, or **Ctrl+B**).
   - **Abort** the loop.
4. Run this **once per invocation**, not per pass.

This mirrors the token-permission preflight pattern — probe the real capability and surface a choice,
rather than launch silently broken.

### 1. Review

Every pass — pass 1 included — gets a **fresh, cold review**: spawn a subagent that runs the actual
`/critical-code-reviewer` skill on the target, exactly as if a brand-new session typed
`/critical-code-reviewer <PR link>`. See [The review subagent](#the-review-subagent) for how. **By
default spawn it in the foreground** (a plain blocking `Agent` call) — prompts pass through and no
preflight is needed. **Only in [watchdog mode](#default-mode-foreground-no-watchdog)** spawn it through
the **[hang watchdog](#hang-watchdog--bound-each-review-spawn)** (background + liveness watchdog +
bounded retry) so a stalled Claude API call can't freeze an unattended loop indefinitely.
The loop driver never reviews in its own context — it triages, fixes, and ships; the skill judges.
That keeps every verdict independent (pass 2+ never inherits your reasoning for the fixes it's
checking) and ends the run cleanly at the `## Verdict` line.

### 2. Post the review comment (Comment A) — always, before acting on the verdict

The moment the review returns its `## Verdict`, post one PR comment using
**`../critical-code-reviewer/templates/pr-comment.md`** — *before* step 3 acts on the verdict:

- **Number:** scan existing `Code Review (Pass N)` review comments and increment (no prior comments →
  Pass 1). Because **every** pass posts this comment before the loop decides anything (step 3), the
  count never drifts — it *is* the loop's pass index.
- **Content:** the cold-review verdict and findings, faithful to what the reviewer said — don't
  pre-empt the fix here. This is the untouched record of the judge's call.

Because step 2 runs before the loop acts on the verdict, **every** pass — Approve, escalation, or
cap — has its review comment on the record for free.

### 3. Act on the verdict

- **`Approve`** → stop and report. **Done** — no fix, so no fixes comment; the step-2 review comment
  is the final record.
- **`Request Changes` / `Needs Discussion`** → go to Triage.

### 4. Triage every Blocking + Required finding

Sort each finding into exactly one bucket:

| Bucket | Criteria | Action |
|---|---|---|
| **Fix now** | In the PR's existing scope **and** one clearly-correct approach **and** non-destructive/reversible **and** no new product/architecture decision | Apply the fix |
| **Escalate** (critical decision) | Any of the [escape criteria](#escape-criteria) below | **Stop the loop, ask the user** |
| **Defer** | Real but out-of-scope, or pre-existing and not introduced by this PR | Don't fix; record in the review comment's Notes / offer `/to-issues` |

If **any** finding lands in **Escalate**, stop and ask the user *before* touching the tree — don't fix
the easy ones and silently sit on the hard one. The step-2 review comment (with the escalated item
written as an open Action Item) is **already posted**, so the pause is on the record; just surface the
decision with options and a recommendation (use `AskUserQuestion`). When you resume after the user
decides: if it unblocks a fix, continue to step 5; if it re-scopes the finding away with no code
change and nothing else is in the "Fix now" bucket, there's nothing to commit — skip to step 9 and
spawn pass `N+1` (which gets its own step-2 review comment, correctly numbered).

### 5. Fix (the "Fix now" bucket only)

- **Editing in-scope files is pre-authorized — no per-edit confirmation.** Invoking the loop grants
  `Edit`/`Write` on the files in scope, **including skill/code files under `.claude/`** when the PR
  itself touches them. Don't pause to ask "may I edit this?"; just apply the fix. (This is the *source*
  grant; the harness still needs `Edit`/`Write` allowed in settings to run prompt-free — see
  [Editing permissions](#editing-permissions).)
- Stay inside the **scope guard**: edit only files the PR already touches plus the directly-required
  ripple. A finding that wants a broad refactor is a *Defer* or *Escalate*, not a fix. The pre-auth
  above covers in-scope edits **only** — it never widens the scope guard.
- Match surrounding code style and the project's conventions (CLAUDE.md, glossary).
- The `.claude/` **no-stage** rule (see [Staging temp files](#staging-temp-files--contexttmp-only)) is
  unchanged: it forbids *scratch/temp* files under `.claude/`, not legitimate edits to in-scope
  `.claude/` source that the PR is actually changing.

### 6. Verify before committing

Run the project's check/test gate and make it green:

- **This repo:** run `pnpm check && pnpm test` yourself before pushing — **both**. There's no
  pre-push hook, so nothing re-checks at push time; keeping the bar green is on you (`pnpm test`
  matters most after edits in `src/lib/services/`).
- If a check fails on something **pre-existing, unrelated, or a known environment artifact** (e.g.
  this machine's Windows prettier/audit/test false-failures), note it and move on — do **not** loop
  trying to fix noise you didn't introduce.

### 7. Commit + push

- Commit only the files your fixes touched (`git add <paths>` — never `git add -A` into someone
  else's uncommitted work). Reference the findings being resolved in the message.
- End the commit body with the repo's current `Co-Authored-By` trailer (the model-specific Claude
  Code form the project actually uses) — note this differs from the generic value hardcoded in
  `/ship`, so don't blindly inherit `/ship`'s trailer.
- Push to the PR branch. Use `/ship` if branch/PR plumbing is needed; a straight commit + push is
  fine when the branch and PR already exist.

### 8. Post the fixes comment (Comment B) — only when the pass committed

After the push, post a **second** PR comment that records what this pass changed:

- **Heading:** `Pass N — Fixes applied` (reuse this pass's `N` from step 2 — *not* a new
  `Code Review (Pass N)` heading, so the step-2 numbering scan stays clean).
- **Content:** what you changed and the commit SHA(s), tying each resolved finding → fix → commit, so
  the next reader sees how the step-2 findings were addressed.
- **Skip it** only when the pass made no commit at all (a pure-Approve pass, or an escalation/defer
  that changed nothing) — there's nothing to record.

### 9. Loop or cap

- If `N` reached `max-passes` without an Approve → **stop and escalate** with a status dump:
  outstanding findings, what you fixed, and why it hasn't converged (the step-2 review comment is
  already up; add the step-8 fixes comment if you committed this pass). Don't silently keep going.
- **Oscillation guard:** if a fix re-opens a finding a prior pass closed, or two passes disagree on
  whether something is real, stop and ask — that's a decision, not a fix.

## Escape criteria

Stop the loop and bring the decision to the user when a finding's fix would:

1. Have **material design tradeoffs** with no single clearly-correct approach (competing APIs,
   behaviors, perf/complexity tradeoffs).
2. Be **destructive or irreversible** — data/schema migration, deleting content, force-push,
   history rewrite, dependency removal.
3. **Expand scope** beyond the PR's intent — a new feature, a cross-cutting refactor, renames that
   ripple widely.
4. **Contradict a documented decision** — CLAUDE.md, an ADR, the glossary, or the PR's own stated
   intent.
5. Touch **security, secrets, licensing, or auth** in a way that needs human sign-off.
6. **Oscillate** — re-open a closed finding, or pass-to-pass disagreement on what's real.

Default to escalating when genuinely unsure. The value of this loop is that it *doesn't* guess on
the calls a human should make.

## The review subagent

Each pass's review runs in a **fresh subagent** (Agent tool, `general-purpose`) that is the
equivalent of a brand-new session running `/critical-code-reviewer <PR link>`. Hand it just the
target (PR URL / number, or branch) and the repo path — **nothing else**. No prior findings, no fix
rationale, no running commentary from this session: inheriting any of that is exactly what defeats
cold eyes.

- **It invokes the skill — it does not paraphrase it.** The review logic has one source of truth:
  the `critical-code-reviewer` skill. Tell the subagent to run `/critical-code-reviewer` on the
  target. If the spawned agent can't load the slash command, have it read
  `.claude/skills/critical-code-reviewer/SKILL.md` and execute it **verbatim** as its complete
  instructions — identical behavior, since the skill *is* that prompt file. Never re-summarize the
  methodology into the subagent's prompt; a copy silently drifts the moment the skill changes.
- The skill already suppresses its Next Steps / interactive flow when run as a subagent, so the
  review ends at the `## Verdict` line — which is what the loop reads.
- There is no separate "re-verify the old findings" step: a cold review of the **current file
  state** (not just the diff) covers it — if a fix didn't land, the reviewer surfaces it again on
  its own.
- **Don't flag harness built-ins as missing.** Built-in Claude Code commands and agent types —
  `/goal`, `general-purpose`, `Explore`, the `Skill` tool, etc. — are provided by the harness, not
  defined in the repo. "I couldn't find it under `.claude/`" is **not** evidence it doesn't exist;
  check against Claude Code's built-ins before calling a reference dead. (Dogfooding this loop, two
  cold passes wrongly flagged `/goal` and `general-purpose` exactly this way.)
- The subagent outputs **only** the review ending in its `## Verdict` line (its final text is the
  return value; no next-steps section, no questions). It must **not rubber-stamp and not manufacture
  problems** — Approve iff there are genuinely no blocking issues.

Cross-pass memory lives in the loop, not the subagent: *you* compare each cold verdict against prior
passes to catch oscillation (see [Loop or cap](#9-loop-or-cap)).

## Hang watchdog — bound each review spawn *(opt-in; off by default)*

> **Off by default.** The loop runs reviews in the foreground (see
> [Default mode](#default-mode-foreground-no-watchdog)). Everything below applies **only** when the
> user opts into watchdog mode (asks for it, or passes `--watchdog`). In that mode, also run
> [Step 0 (permission preflight)](#0-permission-preflight--once-before-pass-1).

The review is the one step that runs **unattended and repeatedly**, so it's where a stalled Claude
API call can freeze the whole loop. An *inference hang* is the call **between the subagent's turns**
never returning: the subagent is suspended waiting on the API, so it **cannot** time itself out (it
isn't running to check a clock). A timeout therefore has to wrap the spawn from **outside**. This
changes only **how the review is launched and supervised** — the review's behaviour, prompt, and
`## Verdict` output are unchanged.

> **Prerequisite — pre-allow the review's tools.** A *background* subagent **auto-denies any tool
> call that would otherwise prompt** (it can't ask). The review runs `gh` / `git` (read-only),
> `Read`, `Grep`, and the `/critical-code-reviewer` `Skill`; if the session doesn't already permit
> those, the backgrounded review **silently degrades** (e.g. can't fetch the diff) instead of
> prompting. So either (a) ensure the session already grants them — `permissions.allow` in
> `settings.json`, or an `auto` / `bypassPermissions` permission mode — or (b) if a background review
> comes back degraded or permission-failed, **re-spawn that pass in the foreground** (prompts pass
> through) and rely on the manual stop below if it then hangs. See the permissions summary at the end
> of this section.

Replace the plain (blocking) `Agent` spawn in step 1 with **background spawn + liveness watchdog +
bounded retry**:

1. **Spawn in the background.** Call the review `Agent` with `run_in_background: true` (same prompt as
   in [The review subagent](#the-review-subagent)). It returns immediately with a **task id** and an
   **output/transcript file path** — keep both.

2. **Arm the watchdog.** Start a `Monitor` on that transcript file that fires only on **silence** —
   not on elapsed time. A genuinely long review keeps appending events (one transcript line per tool
   call / reply, every few seconds); a hang appends nothing. So total runtime can't tell them apart,
   but **gap-since-last-event** can:
   ```bash
   f="<transcript-path-from-step-1>"
   # `stat -c %Y` is GNU coreutils (Git Bash on Windows ships it — correct for this repo's win32 env).
   # On macOS/BSD `stat` use `stat -f %m` instead, or `-c %Y` errors and leaves `age` unset.
   while true; do
     if [ -f "$f" ]; then age=$(( $(date +%s) - $(stat -c %Y "$f") )); else age=0; fi
     if [ "$age" -gt 300 ]; then echo "STALE ${age}s — review subagent silent, likely an inference hang"; break; fi
     sleep 30
   done
   ```
   (`description`: "review-subagent liveness"; `persistent`: false; `timeout_ms` ≈ 900000 — both
   required by the `Monitor` schema.) Threshold = **5 min with no new
   transcript line** = hung. Tune 300s up/down if reviews legitimately pause longer. **Best-effort:**
   the transcript path (the agent's `run_in_background` output file) is written per-event today but is
   an *undocumented* implementation detail — if it ever stalls for a genuinely-working review, widen
   the threshold or fall back to the manual stop (`/agents` **Running** tab, or **Ctrl+B**).

3. **Race the outcomes:**
   - **Review completes first** → you're re-invoked with its result. `TaskStop` the monitor, then read
     the `## Verdict` from the **Agent result** — do **not** Read the transcript file (for a local
     agent it's the full JSONL and will overflow context). Continue to loop step 2.
   - **Monitor emits `STALE` first** → `TaskStop` the review task, record the stall (surface it in your
     status output **and** append a line to a `.context/.tmp/` hang-trace file so an unattended hang
     leaves an inspectable artifact), then **re-spawn** from watchdog step 1.
   - **Monitor exits without firing `STALE`** (it hit `timeout_ms` while the review is still running, or
     the transcript file never appeared so `age` stayed `0`) → the watchdog is now dead but the review
     isn't. Re-arm a fresh `Monitor` on the same transcript path and keep waiting; the review will still
     re-invoke you on completion (the first outcome). If the file truly never appears across two
     re-arms, fall back to the manual stop (`/agents` **Running** tab / **Ctrl+B**).

4. **Cap retries at 2.** If the review stalls on two consecutive spawns, **stop and escalate**: "the
   review subagent keeps stalling — almost certainly a platform inference hang (see
   `.context/bugs/harness/subagent-inference-call-hang.md`); re-run `/review-loop` later or check
   Anthropic status." Don't burn passes chasing it.

This is a stopgap for a harness gap (there's no `timeout` on the `Agent` tool yet); the real fix is a
per-turn timeout in the harness/Agent layer.

### Permissions this adds

Two groups — the **background review** must have its tools pre-granted (else it auto-denies and
degrades), and the **driver's watchdog** needs the supervision tools:

```jsonc
// settings.json → "permissions": { "allow": [ ... ] }
// (1) Tools the BACKGROUND review subagent invokes — pre-allow so they don't auto-deny:
"Read", "Grep", "Glob",          // read-only; usually already allowed
"Skill",                          // to run /critical-code-reviewer
"Bash(gh pr view:*)", "Bash(gh pr diff:*)", "Bash(gh api:*)",
"Bash(git diff:*)", "Bash(git log:*)", "Bash(git show:*)", "Bash(git status:*)",

// (2) Tools the DRIVER uses to supervise the spawn:
"Agent",                          // spawn the review (run_in_background)
"Monitor",                        // the liveness watchdog
"TaskStop",                       // kill a hung review
"Bash(stat:*)", "Bash(date:*)", "Bash(sleep:*)"   // the Monitor poll command
```

Alternatively, run the unattended loop under an `auto` or `bypassPermissions` permission mode instead
of enumerating these (broader, but no per-tool allow-list to maintain). If you'd rather not grant any
of the above, keep the review **foreground** (today's behaviour) and accept that a hang needs a manual
stop (`/agents` **Running** tab / **Ctrl+B**) — the watchdog only works with a background spawn.

## Termination

The loop ends on exactly one of — each having **already posted that pass's review comment** in step 2
(which runs before the verdict is acted on, so it can't be skipped):

- **Approve** — verdict is Approve; the step-2 review comment is the final record; no fix, so no fixes
  comment; report success.
- **Escalation** — a finding hit an escape criterion; the step-2 review comment (escalated item as an
  open Action Item) is already up; ask the user; loop paused.
- **Pass cap** — `max-passes` reached without Approve; the step-2 review comment is up (plus the
  step-8 fixes comment if this pass committed); report status and stop.

On every termination, give the user a tight summary: passes run, what was fixed and pushed (with
commit SHAs), the final verdict, and anything outstanding (deferred findings, escalations).

## Optional: long-run hardening

The loop lives in these instructions, so it self-drives in one flow and needs no external machinery.
For very long or unattended runs where a context hiccup could make the agent bail early, the loop
*may* set a goal with the built-in **`/goal`** command ("don't stop until verdict is Approve or a
decision is escalated") as a backstop. `/goal` is a **built-in Claude Code slash command** — "set a
goal Claude checks before stopping" — **not** a user-defined skill, so don't go hunting for it under
`.claude/skills/` and don't flag it as missing; it ships with Claude Code. This is
belt-and-suspenders, not required — and it must still honour the escape criteria (escalation is a
valid stop).

## Editing permissions

The Step-5 pre-auth above tells *the loop* not to ask before editing in-scope files. But a markdown
grant can't suppress a **harness** permission prompt — that's governed by `settings.json` /
`settings.local.json`. For the loop to actually run **prompt-free on edits**, the session must allow
the edit tools:

```jsonc
// .claude/settings.local.json → "permissions": { "allow": [ ... ] }
"Edit", "Write"
```

- These are added to this project's `settings.local.json`. The grant is **project-wide** (every session
  in this repo), not scoped to the loop — permissions live in settings, and skills can't carry their
  own enforced scope. The Step-5 **scope guard** is what keeps the loop's edits bounded to the PR.
- **Caveat:** edits to paths under `.claude/` may still trip the harness's built-in *sensitive-file*
  confirmation even with `Edit`/`Write` allowed. If a legitimate in-scope `.claude/` edit prompts mid-
  run, that's the harness, not this skill — approve it once, or run under an `auto` /
  `bypassPermissions` permission mode for a fully unattended pass.

## Never

- **Never merge** — merging is always a human action.
- **Never** `--no-verify`, force-push, or rewrite history without explicit, in-the-moment user
  permission.
- **Never** commit unrelated uncommitted working-tree changes you didn't make.
- **Never** use GitHub closing keywords (`Closes`/`Fixes`/`Resolves`) in commits or PR comments —
  use `Related to #N`.
- **Never** keep looping to manufacture an Approve — converge by fixing real issues or escalate.
- **Never** stage temp files (comment bodies, commit messages) under `.claude/` or `.git/` — use
  `.context/.tmp/` and clean up there (see "Staging temp files").
