---
sidebar_position: 4
title: File URLs
---

# File URLs

*Why S3 links are stripped, and how to fetch the bytes.*

URLs in `files` properties and in Markdown body embeds (image, PDF, video, file, audio) may have had their **AWS S3 pre-signed query string stripped**:

- **Original** (from the Notion API): `https://prod-files-secure.s3.us-west-2.amazonaws.com/<bucket>/<uuid>/file.pdf?X-Amz-Algorithm=...&X-Amz-Signature=...&X-Amz-Date=...`
- **On disk** (default): `https://prod-files-secure.s3.us-west-2.amazonaws.com/<bucket>/<uuid>/file.pdf`

**Why:** Notion rotates `X-Amz-Signature` every hour. Without stripping, every refresh produces a huge noisy diff even when nothing changed.

## What this means

- The path (file UUID + filename) **is stable** — use it as the file's identifier.
- The stripped URL **won't return file bytes** on a direct GET — AWS rejects unsigned requests to `prod-files-secure`.
- To fetch the bytes, re-query the Notion API for the parent page and use the freshly signed URL it returns.
- With `--keep-presigned-params`, URLs keep the auth string, but the signature is **already expired** (1-hour TTL).

External URLs (user-set "external" file references, not uploaded to Notion) are never stripped.

To strip URLs in a workspace synced before this behavior existed, run [`clean`](commands.md#clean).
