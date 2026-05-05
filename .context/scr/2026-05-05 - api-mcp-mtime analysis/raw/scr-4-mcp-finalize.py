"""scr-4 MCP path — finalize: wait into next minute, read after-timestamps,
compare to baselines, merge with API results, print final table."""

import json
import os
import sys
import time
import urllib.request

API_KEY = os.environ.get("NOTION_SYNC_API_KEY")
BASE = "https://api.notion.com/v1"
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Notion-Version": "2022-06-28",
    "Content-Type": "application/json",
}


def get_lt(page_id):
    r = urllib.request.Request(BASE + f"/pages/{page_id}", headers=HEADERS)
    with urllib.request.urlopen(r) as resp:
        return json.loads(resp.read())["last_edited_time"]


def archive(page_id):
    body = json.dumps({"archived": True}).encode()
    r = urllib.request.Request(BASE + f"/pages/{page_id}", data=body, method="PATCH", headers=HEADERS)
    try:
        urllib.request.urlopen(r).read()
    except Exception as e:
        print(f"archive {page_id}: {e}", file=sys.stderr)


def wait_into_next_minute(buffer_s=8):
    start_min = int(time.time()) // 60
    while True:
        now = time.time()
        cur_min = int(now) // 60
        sec = now - cur_min * 60
        if cur_min > start_min and sec >= buffer_s:
            return
        time.sleep(1)


with open("scr-4-mcp-state.json") as f:
    state = json.load(f)

print("Waiting into next minute (M2) before reading afters...", flush=True)
wait_into_next_minute()

print("Reading after-timestamps...", flush=True)
mcp_results = {}
for label, pid in state["pages"].items():
    after = get_lt(pid)
    base = state["baselines"][label]
    bumped = after > base
    mcp_results[label] = {"baseline": base, "after": after, "bumped": bumped}
    print(f"  {label:18s} baseline={base}  after={after}  bumped={bumped}", flush=True)

# files-set was not run via MCP (needs uploaded file IDs)
mcp_results["files-set"] = {"baseline": None, "after": None, "bumped": None, "note": "skipped: MCP needs file IDs from upload"}

# Load API results
with open("scr-4-results-api.json") as f:
    api_state = json.load(f)
api_results = {label: bumped for (label, _, _, bumped) in api_state["results"]}

# Final merged table
labels_order = [
    "title", "rich_text", "number", "select", "multi_select", "date",
    "checkbox", "url-set", "url-clear", "email", "phone_number",
    "relation-set", "relation-clear", "files-set", "files-clear",
]

print("\n=== FINAL: bumps last_edited_time? ===")
print(f"{'property':18s} | {'API':5s} | {'MCP':5s}")
print("-" * 36)
for label in labels_order:
    api_b = "Yes" if api_results.get(label) else ("No" if api_results.get(label) is False else "?")
    mcp_b_raw = mcp_results.get(label, {}).get("bumped")
    if mcp_b_raw is None:
        mcp_b = "N/A"
    else:
        mcp_b = "Yes" if mcp_b_raw else "No"
    print(f"{label:18s} | {api_b:5s} | {mcp_b:5s}")

with open("scr-4-results-mcp.json", "w") as f:
    json.dump(mcp_results, f, indent=2)

print("\nArchiving MCP test pages...")
for pid in state["pages"].values():
    archive(pid)
archive(state["target_id"])
print("Done.")
