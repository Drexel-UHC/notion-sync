package notion

import (
	"net/url"
	"regexp"
	"strings"
)

// notionIDRe matches a Notion page ID in either 32-hex or UUID-with-dashes form.
// The trailing `(?:[^a-f0-9]|$)` boundary prevents the regex from matching the
// first 32 chars of a longer hex run (e.g. a 40-char git SHA) and silently
// truncating the rest. The ID itself is captured in group 1.
var notionIDRe = regexp.MustCompile(`([a-f0-9]{8}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{12})(?:[^a-f0-9]|$)`)

// notionHosts is the set of first-party Notion hosts that CanonicalizeNotionURL
// will rewrite. URLs on any other host pass through unchanged so non-Notion
// inputs (S3 file URLs, GitHub commit URLs, etc.) are never corrupted.
var notionHosts = map[string]bool{
	"notion.so":      true,
	"www.notion.so":  true,
	"app.notion.com": true,
}

// StripPresignedParams removes the AWS S3 pre-signed query string from a URL.
//
// Notion returns file URLs as pre-signed AWS S3 URLs whose query string
// (X-Amz-Signature, X-Amz-Date, X-Amz-Credential, etc.) rotates on every
// API call. The path itself is stable. Stripping the query keeps the
// stable identifier (which file is referenced) while eliminating the
// rotation noise that otherwise churns the diff on every sync.
//
// Returns the input unchanged if it doesn't look like a presigned S3 URL,
// or if it can't be parsed.
func StripPresignedParams(u string) string {
	if u == "" {
		return u
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	if !strings.HasSuffix(parsed.Host, ".amazonaws.com") {
		return u
	}
	q := parsed.Query()
	if q.Get("X-Amz-Signature") == "" {
		return u
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// CanonicalizeNotionURL rewrites any first-party Notion page URL to the
// canonical form `https://app.notion.com/p/{32-hex-no-dashes}`.
//
// Returns the input unchanged for non-Notion URLs (host check), malformed
// inputs, or Notion URLs without an extractable 32-hex page ID in the path.
// Pure pass-through on miss — never errors — so callers can wrap arbitrary
// strings without risk of corrupting non-URL values.
//
// The page ID is taken from the URL path only; query parameters (e.g. `?v=`
// view IDs on database links) and fragments (e.g. `#blockId` anchors) are
// ignored, so a database-view link still canonicalizes to the page itself
// rather than the view ID.
func CanonicalizeNotionURL(s string) string {
	if s == "" {
		return s
	}
	parsed, err := url.Parse(s)
	if err != nil {
		return s
	}
	if !notionHosts[strings.ToLower(parsed.Host)] {
		return s
	}
	matches := notionIDRe.FindAllStringSubmatch(strings.ToLower(parsed.Path), -1)
	if len(matches) == 0 {
		return s
	}
	id := strings.ReplaceAll(matches[len(matches)-1][1], "-", "")
	return "https://app.notion.com/p/" + id
}
