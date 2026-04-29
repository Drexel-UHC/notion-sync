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
