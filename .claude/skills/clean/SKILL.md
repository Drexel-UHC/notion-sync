---
name: clean
description: Clean up merged branches. Use when the user says "clean", "clean up branches", "delete merged branches", or wants to tidy up stale branches. Removes local and remote branches that have already been merged into main.
---

# Clean

Clean up merged branches from both local and remote. Only deletes branches that are fully merged into `main`.

## Step 1: Ensure we're on main

First, check for uncommitted changes with `git status`. If there are uncommitted changes, **warn the user and stop** — do not switch branches with dirty working state.

If clean, and not already on `main`, note the current branch name (it may be a deletion candidate). Then switch:

```bash
git checkout main
git pull origin main
```

## Step 2: Prune stale remote tracking refs

```bash
git fetch --prune
```

This removes local tracking refs for remote branches that no longer exist.

## Step 3: Identify merged local branches

```bash
git branch --merged main
```

Filter out `main` itself (and any other protected branches like `develop`, `staging`, `production`). These are candidates for deletion.

## Step 4: Identify merged remote branches

```bash
git branch -r --merged main
```

Filter out `origin/main`, `origin/HEAD`, and any protected branches. These are candidates for deletion.

## Step 5: Report and decide whether to confirm

Show the user a summary of what will be deleted:

- **Local branches to delete:** list them
- **Remote branches to delete:** list them

If there's nothing to clean up, tell the user everything is already tidy and stop.

**Confirmation logic:** Since `git branch --merged main` and `git branch -d` both guarantee the branch is fully merged, these deletions are safe. **Proceed without asking for confirmation** — just delete them and report the results.

## Step 6: Delete confirmed branches

### Local branches

```bash
git branch -d <branch-name>
```

Use `-d` (not `-D`) to ensure only fully merged branches are deleted.

### Remote branches

```bash
git push origin --delete <branch-name>
```

## Step 7: Report

Tell the user:
- How many local branches were deleted (and their names)
- How many remote branches were deleted (and their names)
- Confirm the repo is clean

## Rules

- **NEVER** delete `main`, `develop`, `staging`, or `production` branches
- **NEVER** use `git branch -D` (force delete) — only `-d` (safe delete)
- **NEVER** switch branches when there are uncommitted changes — warn and stop
- **ALWAYS** show the user what was deleted in the final report
- If a branch fails to delete, report the error and continue with the rest
