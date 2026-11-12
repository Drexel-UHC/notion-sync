---
name: release
description: Tag and publish a new release. Interactive — confirms version, shows changelog, and asks before each step. Use when the user says "release", "cut a release", or "publish a new version".
---

# Release

Interactive, step-by-step release workflow. Asks the user to confirm between each step.

## Step 1: Preflight checks

Run these checks and report results:

1. `git branch --show-current` — must be `main`
2. `git status` — working tree must be clean (no uncommitted changes)
3. `git pull origin main` — ensure up to date with remote

If any check fails, stop and tell the user what to fix. Do NOT continue.

Print: `Step 1: Preflight — PASS`

## Step 2: Show current version and ask for new version

1. Get the latest tag: `git describe --tags --abbrev=0`
2. Show the current version to the user.
3. Use AskUserQuestion to ask what the new version should be. Offer these options:
   - **Patch** bump (e.g. v0.3.0 → v0.3.1)
   - **Minor** bump (e.g. v0.3.0 → v0.4.0)
   - **Major** bump (e.g. v0.3.0 → v1.0.0)
   - Other (user types a custom version)

Compute the suggested versions dynamically from the current tag. Show the computed version in each option's description.

**Validation:** The chosen version must:
- Start with `v` followed by semver (`vX.Y.Z`)
- Be greater than the current version

If validation fails, tell the user and re-ask.

## Step 3: Docs version check

The binary's generated `AGENTS.md` links to `/<version>/` on the docs site, so a release
with no matching docs version ships dead links. This skill never cuts the docs itself — it
only checks.

1. Read `docusaurus/versions.json` and check that it lists `<version>` (e.g. `v2.2.0`).
2. **Listed** → print `Step 3: Docs version — PASS` and continue.
3. **Missing** → stop the release. Tell the user to cut the docs version in its own PR, merge
   it, then re-run `/release`:
   - `cd docusaurus && pnpm docusaurus docs:version <version>`
   - in `versioned_docs/version-<version>/`, replace every `/notion-sync/next/` link with
     `/notion-sync/<version>/`
   - `pnpm build` must pass

Do NOT cut the docs, commit, or tag in this step.

## Step 4: Show changelog and confirm

1. Run `git log <current-tag>..HEAD --oneline` to get all commits since the last release.
2. Display the changelog to the user formatted as a bullet list.
3. Display: `Release: <current-version> → <new-version>`
4. Use AskUserQuestion to confirm:
   - **Yes, tag and release** — proceed
   - **No, abort** — stop

If the user aborts, stop immediately.

## Step 5: Tag and push

1. Create an annotated tag: `git tag -a <version> -m "Release <version>"`
2. Push the tag: `git push origin <version>`

If either command fails, stop and report the error.

## Step 6: Report next steps

Tell the user:

- The tag has been pushed
- GitHub Actions will now automatically:
  - Build binaries for all platforms
  - Create the GitHub Release with auto-generated release notes
  - Update the Scoop manifest and commit it to `main`
- Link to the Actions run: `https://github.com/ran-codes/notion-sync/actions`
- Remind them to check the release page after CI finishes

## Rules

- **NEVER** skip a confirmation step — always ask before proceeding
- **NEVER** tag on any branch other than `main`
- **NEVER** force push tags
- If something fails, stop and tell the user — don't retry
