package notion

import "testing"

func TestStripPresignedParams(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "notion presigned URL gets stripped",
			input: "https://prod-files-secure.s3.us-west-2.amazonaws.com/abc/uuid/file.pdf?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAZI2LB466ZDHWKQMA%2F20260428%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20260428T150234Z&X-Amz-Signature=9af1bc&X-Amz-SignedHeaders=host",
			want:  "https://prod-files-secure.s3.us-west-2.amazonaws.com/abc/uuid/file.pdf",
		},
		{
			name:  "amazonaws URL without signature is left alone",
			input: "https://example.s3.amazonaws.com/path/file.pdf?foo=bar",
			want:  "https://example.s3.amazonaws.com/path/file.pdf?foo=bar",
		},
		{
			name:  "non-AWS URL is left alone",
			input: "https://example.com/file.pdf?X-Amz-Signature=abc",
			want:  "https://example.com/file.pdf?X-Amz-Signature=abc",
		},
		{
			name:  "external user-supplied URL untouched",
			input: "https://my-cdn.example.org/static/thumbnail.png",
			want:  "https://my-cdn.example.org/static/thumbnail.png",
		},
		{
			name:  "malformed URL passes through",
			input: "://not a url",
			want:  "://not a url",
		},
		{
			name:  "presigned URL with fragment also strips fragment",
			input: "https://prod-files-secure.s3.us-west-2.amazonaws.com/file.pdf?X-Amz-Signature=xyz#frag",
			want:  "https://prod-files-secure.s3.us-west-2.amazonaws.com/file.pdf",
		},
		{
			name:  "subdomain match — only suffix .amazonaws.com counts",
			input: "https://evil-amazonaws.com/file.pdf?X-Amz-Signature=xyz",
			want:  "https://evil-amazonaws.com/file.pdf?X-Amz-Signature=xyz",
		},
		{
			name:  "real Notion thumbnail with full presigned params",
			input: "https://prod-files-secure.s3.us-west-2.amazonaws.com/e017b0f8-3df0-46b2-a9d7-b49a0c4582cf/697225d9-5e52-4d4f-a123-6d83e2c21600/Screenshot_2024-06-10_115549.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Content-Sha256=UNSIGNED-PAYLOAD&X-Amz-Credential=ASIAZI2LB466V63O2FJB%2F20260428%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20260428T180102Z&X-Amz-Expires=3600&X-Amz-Security-Token=IQoJb3JpZ2luX2VjEBoaCXVzLXdlc3QtMiJHMEUCIFBJgHQRRvPhYUyh1DTcxAJvrh357AafZTapn0iZ47PeAiEA%2B3o5p5BwR%2BdZCcSB0nOhUK7C9vHUDXMoj%2BFzBPyBVg8qiAQI4%2F%2F%2F%2F%2F%2F%2F%2F%2F%2F%2FARAAGgw2Mzc0MjMxODM4MDUiDLqAAgPdgKEyE6Fv7CrcA0d2Y40zMuigkZWOQHRXLMdRIKytc308NQYfpk0CdyXUCfNXOkAN9qgOD0WpHbZcT0TFhggrh%2F%2B2OEao7ijTacdF9VnMiswsREyXna%2Fd5YFU%2Fr6EfPiK%2BVY3sJxSCmjozzOcoZRDRufI6ctgtHGQXdO2BAEIHcoajA80qhAGuCCZpXnjo4aV%2FuLfM0kDVH9tlWfRJ6t2mvcaXjIhwKu5h7RPYC1fX%2F9uWF2h65Odsdk4dLFG3QMzEGOe56YNCbRGFqzGqPspQSkArjSusrsb%2BS4c8JjvC0s2p2MXB%2FIbGJSl%2Fo91NrurUNi8%2FXht57qRZ7nJpRhMgmlt%2FEC65jrVfqng4Ggydv%2B6itP0343EQjq4L8eWEfCVRXMBkMC39aDSTQo9lM9KNceKqaIiXw%2BQNgtFAIyq8s35rlg%2B96kDDS00%2B1qffH7L8y1CFdT4ln74vhulxgmt%2F4imvYP9DH5dmNqABPFjPX9UMe4NxxWcFMIqS45zr2dmrSFK7rvHv%2FoO2sOO6cQjs3s2WXy0EHLjLgxmeH%2BDx5txiEN9wn5kH3DdX%2Fa5HqJd1tnfCaXpw2OfqULEEL04kJbUY7qIK0pugzhXzPHvxPiAm46xmADkGHCY%2BZjvyVKF2%2FEnu3BhMPHiw88GOqUByg6FIddKTC0tdx0xho2ssUNaz%2BJHDnamElGsIM5FJxudi%2B5EQjwWGL9oM8tYJyOQQQ2%2F%2BfP%2BaPFzkMpISqLLmEIFvIqzVZUFnK0krM5QDtFNqZlO5iswq9WuDUM6Yw7MlYLkhHywhcAkkNRj1NXmhucwgN%2Bwq6%2BD5bYWA5w%2FgEScloNonIFqBHRaCygLcppUIB5OZopFAsB%2FWChDn8c%2BHHP0DbyT&X-Amz-Signature=76111bc0c2ea986ea80f957c0f6a52f0e9691ffe2cdc1924b4fdaa8ecc200ed4&X-Amz-SignedHeaders=host&x-amz-checksum-mode=ENABLED&x-id=GetObject",
			want:  "https://prod-files-secure.s3.us-west-2.amazonaws.com/e017b0f8-3df0-46b2-a9d7-b49a0c4582cf/697225d9-5e52-4d4f-a123-6d83e2c21600/Screenshot_2024-06-10_115549.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripPresignedParams(tt.input)
			if got != tt.want {
				t.Errorf("StripPresignedParams(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCanonicalizeNotionURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "legacy notion.so with title slug",
			in:   "https://www.notion.so/Title-1234567890abcdef1234567890abcdef",
			want: "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
		},
		{
			name: "already canonical — idempotent",
			in:   "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
			want: "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
		},
		{
			name: "bare notion.so without title slug",
			in:   "https://notion.so/1234567890abcdef1234567890abcdef",
			want: "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
		},
		{
			name: "uuid-with-dashes in path — collapses to no-dashes",
			in:   "https://www.notion.so/Title-12345678-90ab-cdef-1234-567890abcdef",
			want: "https://app.notion.com/p/1234567890abcdef1234567890abcdef",
		},
		{
			name: "non-Notion URL passes through unchanged",
			in:   "https://example.com/some/path",
			want: "https://example.com/some/path",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "notion URL without an extractable hex ID",
			in:   "https://www.notion.so/Some-Workspace/SettingsPage",
			want: "https://www.notion.so/Some-Workspace/SettingsPage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalizeNotionURL(tt.in); got != tt.want {
				t.Errorf("CanonicalizeNotionURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
