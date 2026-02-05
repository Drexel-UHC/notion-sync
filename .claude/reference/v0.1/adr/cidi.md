# CI/CD — What It Is and How It Works Here

## What is CI/CD?

CI/CD stands for **Continuous Integration / Continuous Delivery**. It's just automated stuff that runs when you push code to GitHub.

Think of it like a robot that watches your repo. Every time you push code or open a PR, the robot:

1. Grabs your code
2. Builds it
3. Runs your tests
4. Tells you if anything broke

That's CI. The "CD" part is when the robot also *ships* your code (builds binaries, creates a release, etc).

## What We Have: Two Workflows

We have two GitHub Actions workflow files in `.github/workflows/`. Each one is a YAML file that tells GitHub's robots what to do.

### 1. `test.yml` — The CI Part

**When does it run?** Every push to `main` and every pull request targeting `main`.

**What does it do?**

```
Step 1: Check out the code        (git clone, basically)
Step 2: Install Go 1.22            (set up the build environment)
Step 3: go vet ./...               (catch obvious bugs — like a linter)
Step 4: go test ./...              (run all your tests)
Step 5: go build ./cmd/notion-sync (make sure it compiles)
```

If any step fails, GitHub shows a red X on the PR. If they all pass, green checkmark.

**Why does this matter?** It catches broken code *before* it gets merged. You don't have to remember to run tests locally — the robot does it for you.

### 2. `release.yml` — The CD Part

**When does it run?** Only when you push a git tag that starts with `v` (like `v0.1.0`).

**What does it do?**

```
Step 1: Check out the code
Step 2: Install Go 1.22
Step 3: Build 5 binaries:
        - Windows (amd64)
        - macOS (amd64 — Intel Macs)
        - macOS (arm64 — M1/M2/M3 Macs)
        - Linux (amd64)
        - Linux (arm64)
Step 4: Create a GitHub Release with all 5 binaries attached
```

So when you do `git tag v0.1.0 && git push --tags`, GitHub automatically builds binaries for every platform and publishes them on the Releases page. Users can then download the right binary for their OS.

## How to Trigger Each One

| Workflow | Trigger | You do... |
|----------|---------|-----------|
| `test.yml` | Push to main or open PR | Just push code or open a PR. Automatic. |
| `release.yml` | Push a `v*` tag | `git tag v0.1.0` then `git push --tags` |

## Key Concepts

**GitHub Actions** — GitHub's free CI/CD service. Runs your workflows on their servers.

**Workflow** — A YAML file in `.github/workflows/` that defines what to automate.

**Job** — A unit of work inside a workflow. Our test workflow has one job. Our release workflow has two (build + release).

**`runs-on: ubuntu-latest`** — Every job runs on a fresh Linux VM. It's a clean machine every time — nothing left over from previous runs.

**`CGO_ENABLED=0`** — In the release workflow, this means "don't link to C libraries." This makes the binary fully self-contained — users just download and run it, no dependencies needed.

**Matrix build** — The release workflow uses a `matrix` strategy to build the same code 5 times with different OS/architecture targets, in parallel. Instead of writing the build step 5 times, you write it once and say "do this for each combo."

## The Flow in Practice

```
You write code
    |
    v
Push to a branch, open PR
    |
    v
test.yml runs automatically
    |
    ├── Tests pass  -> Green checkmark, safe to merge
    └── Tests fail  -> Red X, fix your code

    ... later ...

Merge to main
    |
    v
Ready to release? Tag it:
    git tag v0.1.0
    git push --tags
    |
    v
release.yml runs automatically
    |
    v
Binaries appear on GitHub Releases page
    |
    v
Users download and use your tool
```
