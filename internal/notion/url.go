package notion

import (
	"net/url"
	"strings"
)

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
