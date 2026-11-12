#!/usr/bin/env bash
# Claude Code status line: model name + git branch + context bar + logged-in account
# No external dependencies (no jq required)

input=$(cat)

# Extract model name
model=$(echo "$input" | grep -o '"display_name" *: *"[^"]*"' | head -1 | sed 's/.*: *"//;s/"//')
[ -z "$model" ] && model="Claude"

# Git branch
branch=$(git branch --show-current 2>/dev/null)

# Repo home page on GitHub, from origin (https, ssh, or scp-style); empty for any other host
repo_url=$(git remote get-url origin 2>/dev/null | sed -nE 's#^(https://|ssh://)?([^@/]+@)?github\.com[:/]([^/]+/[^/]+)$#https://github.com/\3#p' | sed 's#\.git$##')

# Logged-in account: org name from ~/.claude.json (rewritten on /login), else email.
# One file per machine, so this is the last login, not necessarily this session's.
account=""
claude_json="${CLAUDE_CONFIG_DIR:-$HOME}/.claude.json"
if [ -f "$claude_json" ]; then
  account=$(grep -o '"organizationName" *: *"[^"]*"' "$claude_json" | head -1 | sed 's/.*: *"//;s/"//')
  [ -z "$account" ] && account=$(grep -o '"emailAddress" *: *"[^"]*"' "$claude_json" | head -1 | sed 's/.*: *"//;s/"//')
fi
# A personal org is named "<email>'s Organization"; keep just the email
account="${account%"'s Organization"}"
account_part=""
[ -n "$account" ] && account_part=" (${account})"

# Extract used_percentage and total_input_tokens (both under context_window)
used=$(echo "$input" | grep -o '"used_percentage" *: *[0-9.]*' | head -1 | sed 's/.*: *//')
tokens=$(echo "$input" | grep -o '"total_input_tokens" *: *[0-9]*' | head -1 | sed 's/.*: *//')

# Branch link target: the branch's open PR when it has one, else the repo home page.
# gh is a network call, so its answer is cached per branch for a minute and refreshed in the
# background: a render never waits on it, and a new PR shows up a refresh or two later.
link_url="$repo_url"
if [ -n "$repo_url" ] && [ -n "$branch" ] && command -v gh >/dev/null 2>&1; then
  cache_dir="${TMPDIR:-/tmp}/claude-statusline"
  key=$(printf '%s|%s' "$(git rev-parse --show-toplevel 2>/dev/null)" "$branch" | cksum | cut -d' ' -f1)
  cache="$cache_dir/pr-$key"
  if [ -z "$(find "$cache" -mmin -1 2>/dev/null)" ]; then
    mkdir -p "$cache_dir" 2>/dev/null
    touch "$cache" 2>/dev/null   # claim the refresh, so overlapping renders spawn one gh
    (
      pr=$(gh pr view "$branch" --json url,state -q 'select(.state == "OPEN") | .url' 2>/dev/null)
      printf '%s' "$pr" > "$cache.$$" && mv "$cache.$$" "$cache"
    ) >/dev/null 2>&1 &
  fi
  pr_url=$(cat "$cache" 2>/dev/null)
  [ -n "$pr_url" ] && link_url="$pr_url"
fi

# Branch, suffixed with the PR number (read off the cached /pull/<N> URL) when it has one
label="$branch"
pr_num=$(printf '%s' "$pr_url" | sed -nE 's#.*/pull/([0-9]+).*#\1#p')
[ -n "$pr_num" ] && label="$branch (PR #$pr_num)"

# Branch, linked when origin is on GitHub
branch_part=""
branch_text="$label"
# OSC 8 hyperlink: clickable in terminals that support it, plain text elsewhere
[ -n "$link_url" ] && branch_text="\033]8;;${link_url}\a${label}\033]8;;\a"
[ -n "$branch" ] && branch_part=" \033[35m${branch_text}\033[0m "

# No context figures until the first reply (fresh session, /clear): model, account, branch
if [ -z "$used" ]; then
  printf "%s%b%b" "$model" "$account_part" "$branch_part"
  exit 0
fi

used_int=$(printf "%.0f" "$used")

# Color thresholds
if [ "$used_int" -ge 90 ]; then
  color="\033[31m"   # red
elif [ "$used_int" -ge 70 ]; then
  color="\033[33m"   # yellow
else
  color="\033[32m"   # green
fi
reset="\033[0m"

# Build 10-block progress bar
filled=$(( used_int / 10 ))
empty=$(( 10 - filled ))
bar=""
for i in $(seq 1 $filled); do bar="${bar}█"; done
for i in $(seq 1 $empty);  do bar="${bar}░"; done

# Format token count as 49k / 1.2M
tokens_part=""
if [ -n "$tokens" ]; then
  if [ "$tokens" -ge 1000000 ]; then
    tokens_fmt=$(awk "BEGIN{printf \"%.1fM\", $tokens/1000000}")
  elif [ "$tokens" -ge 1000 ]; then
    tokens_fmt=$(awk "BEGIN{printf \"%dk\", $tokens/1000}")
  else
    tokens_fmt="$tokens"
  fi
  tokens_part=" ${tokens_fmt}"
fi

# Assemble
printf "%s%b%b ${color}context [%s] %d%%%s${reset}" "$model" "$account_part" "$branch_part" "$bar" "$used_int" "$tokens_part"
