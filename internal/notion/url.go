package notion

import (
	"net/url"
	"regexp"
	"strings"
)

// notionIDRe matches a Notion page ID in either 32-hex or UUID-with-dashes form.
// Used by CanonicalizeNotionURL so URLs containing the dashed UUID variant
// (e.g. legacy `notion.so/Title-12345678-90ab-...`) collapse to the same
// canonical no-dashes output.
var notionIDRe = regexp.MustCompile(`[a-f0-9]{8}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{4}-?[a-f0-9]{12}`)

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
// Returns the input unchanged for non-Notion URLs, malformed inputs, or
// strings without an extractable 32-hex Notion ID. Pure pass-through on
// miss — never errors — so callers can wrap arbitrary strings without
// risk of corrupting non-URL values.
func CanonicalizeNotionURL(s string) string {
	matches := notionIDRe.FindAllString(strings.ToLower(s), -1)
	if len(matches) == 0 {
		return s
	}
	id := strings.ReplaceAll(matches[len(matches)-1], "-", "")
	return "https://app.notion.com/p/" + id
}
