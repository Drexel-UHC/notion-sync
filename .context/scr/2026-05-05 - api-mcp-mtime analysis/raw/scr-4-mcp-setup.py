"""scr-4 MCP path — setup phase.

Creates one test page per property type, waits into the next minute, reads
baseline last_edited_time for each, and saves IDs+baselines to a JSON file
that the finalize script will consume.
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

API_KEY = os.environ.get("NOTION_SYNC_API_KEY")
BASE = "https://api.notion.com/v1"
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Notion-Version": "2022-06-28",
    "Content-Type": "application/json",
}
DATABASE_ID = "2fe57008-e885-8003-b1f3-cc05981dc6b0"


def req(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method, headers=HEADERS)
    with urllib.request.urlopen(r) as resp:
        return json.loads(resp.read())


def get_lt(page_id):
    return req("GET", f"/pages/{page_id}")["last_edited_time"]


def create_with(props):
    return req("POST", "/pages", {"parent": {"database_id": DATABASE_ID}, "properties": props})["id"]


def wait_into_next_minute(buffer_s=8):
    start_min = int(time.time()) // 60
    while True:
        now = time.time()
        cur_min = int(now) // 60
        sec = now - cur_min * 60
        if cur_min > start_min and sec >= buffer_s:
            return
        time.sleep(1)


TITLE = lambda s: {"Name": {"title": [{"text": {"content": s}}]}}

target_id = create_with(TITLE("scr-4-mcp-relation-target"))
print(f"target_id={target_id}", flush=True)

# Same property set as the API test. The init_props column establishes a
# baseline value (relevant for *-clear cases). The "kind" tag is forwarded so
# the finalize script knows what update was attempted.
init_specs = [
    ("title",          TITLE("scr-4-mcp-title-init")),
    ("rich_text",      {**TITLE("scr-4-mcp-rich_text"), "Description": {"rich_text": [{"text": {"content": "d1"}}]}}),
    ("number",         {**TITLE("scr-4-mcp-number"), "Score": {"number": 1}}),
    ("select",         {**TITLE("scr-4-mcp-select"), "Category": {"select": {"name": "Research"}}}),
    ("multi_select",   {**TITLE("scr-4-mcp-multi_select"), "Tags": {"multi_select": [{"name": "urgent"}]}}),
    ("date",           {**TITLE("scr-4-mcp-date"), "Due Date": {"date": {"start": "2026-01-01"}}}),
    ("checkbox",       {**TITLE("scr-4-mcp-checkbox"), "Approved": {"checkbox": True}}),
    ("url-set",        TITLE("scr-4-mcp-url-set")),
    ("url-clear",      {**TITLE("scr-4-mcp-url-clear"), "Website": {"url": "https://example.com/x"}}),
    ("email",          {**TITLE("scr-4-mcp-email"), "Contact Email": {"email": "a@x.com"}}),
    ("phone_number",   {**TITLE("scr-4-mcp-phone"), "Phone": {"phone_number": "111"}}),
    ("relation-set",   TITLE("scr-4-mcp-relation-set")),
    ("relation-clear", {**TITLE("scr-4-mcp-relation-clear"), "Related": {"relation": [{"id": target_id}]}}),
    ("files-set",      TITLE("scr-4-mcp-files-set")),
    ("files-clear",    {**TITLE("scr-4-mcp-files-clear"), "Attachments": {"files": [{"name": "b.txt", "type": "external", "external": {"url": "https://example.com/b.txt"}}]}}),
]

print(f"Creating {len(init_specs)} pages...", flush=True)
pages = {}
for label, init in init_specs:
    pages[label] = create_with(init)
    print(f"  {label}: {pages[label]}", flush=True)

print("Waiting into next minute (M1)...", flush=True)
wait_into_next_minute()

print("Reading baselines...", flush=True)
baselines = {}
for label, pid in pages.items():
    baselines[label] = get_lt(pid)
    print(f"  {label}: {baselines[label]}", flush=True)

state = {
    "target_id": target_id,
    "pages": pages,
    "baselines": baselines,
}
with open("scr-4-mcp-state.json", "w") as f:
    json.dump(state, f, indent=2)

print("\n=== READY FOR MCP WRITES ===")
print("Page IDs (use these in MCP update_page calls):")
for label, pid in pages.items():
    print(f"  {label}: {pid}")
