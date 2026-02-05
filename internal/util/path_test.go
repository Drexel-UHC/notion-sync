package util

import "testing"

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello World"},
		{"Hello/World", "Hello-World"},
		{"Hello:World", "Hello-World"},
		{"Hello*World", "Hello-World"},
		{"Hello?World", "Hello-World"},
		{"Hello\"World", "Hello-World"},
		{"Hello<World>", "Hello-World-"},
		{"Hello|World", "Hello-World"},
		{"Hello\\World", "Hello-World"},
		{"  Spaced  ", "Spaced"},
		{"", "Untitled"},
		{"   ", "Untitled"},
		{"///", "---"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeFileName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFileName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		parts    []string
		expected string
	}{
		{[]string{"a", "b", "c"}, "a/b/c"},
		{[]string{"foo"}, "foo"},
		{[]string{}, ""},
		{[]string{"path", "to", "file.md"}, "path/to/file.md"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := JoinPath(tt.parts...)
			if result != tt.expected {
				t.Errorf("JoinPath(%v) = %q, want %q", tt.parts, result, tt.expected)
			}
		})
	}
}
