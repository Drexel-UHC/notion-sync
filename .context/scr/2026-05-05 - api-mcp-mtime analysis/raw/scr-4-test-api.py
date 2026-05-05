"""scr-4 gap test (API path) — parallel, minute-aware.

Notion's `last_edited_time` is quantized to minute precision, so a clean test
needs each write to be in a distinct minute from baseline. We use 15 separate
pages (one per property type), seed each at minute M0 via creation, wait into
M1, do a single test write per page, wait into M2, then compare baselines.

Requires NOTION_SYNC_API_KEY env var.
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

API_KEY = os.environ.get("NOTION_SYNC_API_KEY")
if not API_KEY:
    print("ERROR: NOTION_SYNC_API_KEY not set", file=sys.stderr)
    sys.exit(2)

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
    try:
        with urllib.request.urlopen(r) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        print(f"HTTP {e.code} {method} {path}: {e.read().decode()}", file=sys.stderr)
        raise


def get_lt(page_id):
    return req("GET", f"/pages/{page_id}")["last_edited_time"]


def update(page_id, props):
    return req("PATCH", f"/pages/{page_id}", {"properties": props})


def create_with(props):
    body = {"parent": {"database_id": DATABASE_ID}, "properties": props}
    return req("POST", "/pages", body)["id"]


def archive(page_id):
    try:
        req("PATCH", f"/pages/{page_id}", {"archived": True})
    except Exception as e:
        print(f"archive {page_id} failed: {e}", file=sys.stderr)


def wait_into_next_minute(buffer_s=8):
    """Block until at least buffer_s seconds into a new minute (avoid clock skew)."""
    start_min = int(time.time()) // 60
    while True:
        now = time.time()
        cur_min = int(now) // 60
        sec_in_min = now - cur_min * 60
        if cur_min > start_min and sec_in_min >= buffer_s:
            return
        time.sleep(1)


# Each test: a fresh page is created with `init` properties (setting baseline).
# After M0->M1 minute boundary, we write `update` properties and observe if
# last_edited_time advances past the creation minute.
TITLE = lambda s: {"Name": {"title": [{"text": {"content": s}}]}}

# Relation target needs to exist before we build the test cases that use it
print("Creating relation target page...", flush=True)
target_id = create_with(TITLE("scr-4-relation-target"))
print(f"target_id={target_id}", flush=True)

# (label, init_props, update_props)
test_cases = [
    ("title",
        TITLE("scr-4-title-init"),
        TITLE("scr-4-title-after")),
    ("rich_text",
        {**TITLE("scr-4-rich_text"), "Description": {"rich_text": [{"text": {"content": "d1"}}]}},
        {"Description": {"rich_text": [{"text": {"content": "d2"}}]}}),
    ("number",
        {**TITLE("scr-4-number"), "Score": {"number": 1}},
        {"Score": {"number": 2}}),
    ("select",
        {**TITLE("scr-4-select"), "Category": {"select": {"name": "Research"}}},
        {"Category": {"select": {"name": "Engineering"}}}),
    ("multi_select",
        {**TITLE("scr-4-multi_select"), "Tags": {"multi_select": [{"name": "urgent"}]}},
        {"Tags": {"multi_select": [{"name": "frontend"}]}}),
    ("date",
        {**TITLE("scr-4-date"), "Due Date": {"date": {"start": "2026-01-01"}}},
        {"Due Date": {"date": {"start": "2026-02-01"}}}),
    ("checkbox",
        {**TITLE("scr-4-checkbox"), "Approved": {"checkbox": True}},
        {"Approved": {"checkbox": False}}),
    ("url-set",
        TITLE("scr-4-url-set"),
        {"Website": {"url": "https://example.com/a"}}),
    ("url-clear",
        {**TITLE("scr-4-url-clear"), "Website": {"url": "https://example.com/x"}},
        {"Website": {"url": None}}),
    ("email",
        {**TITLE("scr-4-email"), "Contact Email": {"email": "a@x.com"}},
        {"Contact Email": {"email": "b@x.com"}}),
    ("phone_number",
        {**TITLE("scr-4-phone"), "Phone": {"phone_number": "111"}},
        {"Phone": {"phone_number": "222"}}),
    ("relation-set",
        TITLE("scr-4-relation-set"),
        {"Related": {"relation": [{"id": target_id}]}}),
    ("relation-clear",
        {**TITLE("scr-4-relation-clear"), "Related": {"relation": [{"id": target_id}]}},
        {"Related": {"relation": []}}),
    ("files-set",
        TITLE("scr-4-files-set"),
        {"Attachments": {"files": [{"name": "a.txt", "type": "external", "external": {"url": "https://example.com/a.txt"}}]}}),
    ("files-clear",
        {**TITLE("scr-4-files-clear"), "Attachments": {"files": [{"name": "b.txt", "type": "external", "external": {"url": "https://example.com/b.txt"}}]}},
        {"Attachments": {"files": []}}),
]

print(f"Creating {len(test_cases)} test pages (M0)...", flush=True)
pages = {}
for label, init, _ in test_cases:
    pid = create_with(init)
    pages[label] = pid
    print(f"  {label}: {pid}", flush=True)

print("\nWaiting for next minute boundary (M1)...", flush=True)
wait_into_next_minute()
print("In M1. Reading baselines and applying test writes...", flush=True)

baselines = {}
for label in pages:
    baselines[label] = get_lt(pages[label])

for label, _, upd in test_cases:
    update(pages[label], upd)
    print(f"  wrote {label}", flush=True)

print("\nWaiting for next minute boundary (M2)...", flush=True)
wait_into_next_minute()
print("In M2. Reading after-write timestamps...", flush=True)

afters = {}
results = []
for label, _, _ in test_cases:
    afters[label] = get_lt(pages[label])
    bumped = afters[label] > baselines[label]
    results.append((label, baselines[label], afters[label], bumped))

print("\n=== API path summary ===")
for label, b, a, bumped in results:
    print(f"  {label:18s} baseline={b}  after={a}  bumped={bumped}")

with open("scr-4-results-api.json", "w") as f:
    json.dump({
        "target_id": target_id,
        "pages": pages,
        "results": results,
    }, f, indent=2)

print("\nArchiving test pages...")
for pid in pages.values():
    archive(pid)
archive(target_id)
print("Done.")
