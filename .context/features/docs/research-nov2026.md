# Research: AGENTS.md vs docs site — where does the split go? (Nov 2026)

Context for issue #123. Question: for a CLI tool like notion-sync, what belongs in the
generated `AGENTS.md` (written into the user's synced workspace) vs the Docusaurus site?

## TL;DR

- **Industry consensus (2026): short AGENTS.md + progressive disclosure.** Keep the
  always-loaded file to what applies to *every* session; link everything else.
  Length targets cluster at **60–150 lines**; Anthropic says <200 lines per CLAUDE.md;
  nobody defends 250+.
- **Evidence is mixed on whether context files help at all** — ETH Zurich (AGENTbench)
  found developer-written files give ~+4% pass rate at ~+20% cost; LLM-generated ones
  *hurt* (−3%). Takeaway for us: every line must earn its place; generic/architecture
  prose is net negative.
- **Linked references get read; orphan docs don't.** Direct references from AGENTS.md
  are loaded in 90%+ of relevant sessions; unreferenced docs <10%. So a slim file that
  *links* is not "hiding" the detail — it is the delivery mechanism.
- **What survives inline, per every source:** exact commands, non-obvious tooling,
  "Always / Ask first / Never" boundaries, decision tables that resolve ambiguity
  before the agent acts, and anything the agent can't infer from the files themselves.
- **Our twist, not covered by the literature:** our AGENTS.md is generated into a
  *data* workspace, not a code repo, and the agent may be offline. So the safety tier
  (push guardrails) must stay inline even though the pattern says "link it".

## Findings

### 1. Length — shorter is the default, not a compromise

| Source | Recommendation |
| --- | --- |
| Anthropic, Claude Code memory docs | target **<200 lines** per CLAUDE.md; "longer files consume more context and reduce adherence"; `@imports` do *not* reduce cost (imported files load with the importer) |
| HumanLayer, *Writing a good CLAUDE.md* | **<60 lines** ideal, <300 hard ceiling; frontier models follow ~150–200 instructions total and Claude Code's own system prompt already uses ~50 |
| Augment Code (internal study, ~100-file modules) | **100–150 lines** optimal; gains reverse past that |
| agents.md v1.1 proposal (issue #135) | <500 lines; a commenter cites 335–535 *words* as the sweet spot |
| McMillan preprint (arXiv 2605.10039) | length (25→500 lines) did **not** change rule compliance — cost is the argument for brevity, not adherence |

Our current template: **262 lines**, ~3,000 words. Over every target above.

### 2. Does the file help at all? (ETH Zurich AGENTbench, arXiv 2602.11988)

- 138 tasks, 12 Python repos, Claude Code / Codex / Qwen Code.
- Developer-written AGENTS.md: **+4%** pass rate, **+19–23%** inference cost.
- LLM-generated AGENTS.md: **−3%** pass rate, same cost increase.
- Critique (Vaughan): benchmark was well-documented OSS Python where the agent could
  discover everything itself; doesn't measure multi-session consistency or domains the
  agent can't infer from files. Lulla et al. found *focused, minimal* context cut
  runtime 28%.
- Takeaway: the file pays off only where it tells the agent what it **cannot infer**
  from the folder. For notion-sync that's: push is destructive, halt semantics,
  `create-pending`, S3 URLs won't download, `notion-deleted` is historical. Property
  mapping tables and JSON schemas are inferrable from the files sitting next to it.

### 3. Progressive disclosure — the pattern everyone converges on

- HumanLayer: core inline (purpose, stack, how to verify) + pointers to
  `agent_docs/*.md` read on demand.
- Augment: "keep each reference's scope clear so the agent knows when to pull it in";
  discovery rates — AGENTS.md 100%, direct references 90%+, dir README 80%, nested
  README 40%, orphan docs <10%.
- agents.md v1.1 proposal formalises it: index at session start, load on demand;
  AGENTS.md = behavioural expectations and constraints, SKILL.md/docs = capabilities.
- Anthropic: Skills exist for exactly this — one description of startup cost, body
  loaded when relevant.
- Mintlify / moxiedocs / HF guide on the file taxonomy: **AGENTS.md** = how to work
  *in* this folder; **llms.txt** = curated link index to a docs site; **per-page .md**
  = the detail behind the links. They are complementary layers, not competitors.

### 4. What stays inline — convergent list

Across HumanLayer, Augment, philschmid, Vaughan, agents.md spec:

- **Exact commands** and non-obvious tooling (tools named in AGENTS.md get used ~160×
  more than unnamed ones).
- **Boundaries as three tiers:** Always do / Ask first / Never do. Pair every "don't"
  with a "do instead" — 30+ unpaired warnings halved task completeness in Augment's data.
- **Procedural workflows** (numbered steps) — moved agents from failing to finishing;
  +25% correctness.
- **Decision tables** that resolve ambiguity *before* the agent acts (+25%
  best-practice score). Our halt-class table is exactly this shape.
- **Domain gotchas the files don't reveal.**

What to cut (same sources): directory overviews the agent can discover, code-style
rules (use a linter), generic architecture prose, anything inferrable from the files.

### 5. Docs-site side — llms.txt and versioning

- llmstxt.org: `llms.txt` is a Markdown index "small enough to fit in context; detail
  lives behind the links, fetched only when needed." Per-page `.md` alternates are the
  intended fetch target. `llms-full.txt` is a convenience bundle.
- `docusaurus-plugin-llms` (what we use) emits one set per version. Versioned URLs
  (`/vX.Y.Z/...`) are preferred by agent-docs guides because they let an agent pin the
  version and avoid drift bugs; `llms.txt` itself has no version concept.
- Anthropic's own docs put a "Documentation Index: fetch llms.txt" header at the top of
  every page — the site advertises its own index to agents.

### 6. What the literature does *not* cover — our situation

Every source assumes AGENTS.md lives in a **code repo the agent is editing**, written by
that repo's maintainers. Ours differs in three ways:

1. **It's tool-generated into a data folder.** The "LLM-generated files hurt" finding is
   about *auto-summarised* content, not a curated template — but it's a warning that
   boilerplate the agent could infer from the folder is net negative.
2. **The agent may be offline or unable to fetch URLs.** Progressive disclosure via
   *local* files (`agent_docs/`) assumes the detail is on disk. Ours is on a website.
   So the tier that must never be missing — push guardrails — cannot be link-only.
3. **It's version-stamped to a binary.** The site documents the latest release; the file
   documents the binary that wrote the folder. Links must pin `/{{VERSION}}/` so the two
   agree, with `next` as the fallback for dev builds.

## Recommended split for notion-sync

| Tier | Where | Why |
| --- | --- | --- |
| **Identity** — what this folder is, layout tree, filename rule, "tool-owned, don't edit" | AGENTS.md inline | agent can't infer "generated by notion-sync" from the files |
| **Metadata keys** — `notion-id`, `notion-last-edited`, `notion-deleted`, `notion-last-pushed`, `notion-create` | AGENTS.md inline (one block) | needed to read any file; small |
| **Gotchas the files hide** — soft deletes are historical; S3 URLs are stable IDs but won't download | AGENTS.md inline (2–3 lines each) | not inferrable; cheap |
| **Push guardrails** — Always/Ask/Never, the 5-condition checklist, never silence a guardrail, halt → report, never clear `create-pending` | AGENTS.md inline, verbatim | safety tier; agent may be offline; decision table + numbered workflow are the highest-value shapes per the research |
| **Halt-class table** (class → meaning → fix) | AGENTS.md inline, compressed | decision table that resolves ambiguity before acting |
| **Pointer block** — site URL pinned to `{{VERSION}}`, `llms.txt`, 4–5 named deep links with one-line scope each | AGENTS.md inline | "scope-labelled references get loaded 90%+" |
| Full property mapping table | site `reference/frontmatter.md` | inferrable from files; long |
| File URL / S3 stripping mechanics | site `reference/file-urls.md` | only the one-line gotcha stays inline |
| `_database.json` / `_page.json` schemas | site `reference/metadata-files.md` | the file is right there to read |
| `clean` / `agents-md` / refresh flag details | site `reference/commands.md` | not needed to *read* a workspace |
| Push mechanics, flags, run-summary JSON, row creation steps, rich-text fidelity | site `guides/push.md` | detail behind the guardrails; fetched when actually pushing |

Target: **~80–100 lines**, ~35% of current. Every safety sentence from the current
template survives; only mechanics and reference tables move.

Two things to verify during implementation:

- `Version` on an untagged build — if it's `dev`, `/{{VERSION}}/` 404s → fall back to
  `/next/`.
- Keep the `{{VERSION}}` placeholder (tests assert it) and the version-stamp comment.

## Sources

- agents.md spec — https://agents.md/
- agents.md v1.1 proposal (progressive disclosure) — https://github.com/agentsmd/agents.md/issues/135
- Anthropic, Claude Code memory / CLAUDE.md & AGENTS.md — https://code.claude.com/docs/en/memory
- HumanLayer, *Writing a good CLAUDE.md* — https://www.humanlayer.dev/blog/writing-a-good-claude-md
- Augment Code, *A good AGENTS.md is a model upgrade* — https://www.augmentcode.com/blog/how-to-write-good-agents-dot-md-files
- Phil Schmid, *Writing a good AGENTS.md* — https://www.philschmid.de/writing-good-agents
- Gloaguen et al. (ETH Zurich / LogicStar), *Evaluating AGENTS.md* — arXiv 2602.11988
- Vaughan, critique of the ETH paper — https://codex.danielvaughan.com/2026/04/13/what-eth-zurich-paper-gets-wrong-right-about-agents-md/
- Dunlop, *CLAUDE.md best practices: what the evidence supports* (McMillan arXiv 2605.10039; Agent READMEs arXiv 2511.12884) — https://www.alexdunlop.com/writing/claude-md-best-practices
- llms.txt proposal — https://llmstxt.org/
- Mintlify, *AGENTS.md vs CLAUDE.md* — https://www.mintlify.com/library/agents-md-vs-claude-md
- Hugging Face, *AI agent documentation: llms.txt, AGENTS.md, SKILL.md, MCP* — https://huggingface.co/blog/phranzia/ai-agent-documentation-guide
- docusaurus-plugin-llms — https://github.com/rachfop/docusaurus-plugin-llms
