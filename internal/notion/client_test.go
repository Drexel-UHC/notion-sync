package notion

import "testing"

func TestNormalizeNotionID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "32-char hex",
			input:    "1234567890abcdef1234567890abcdef",
			expected: "12345678-90ab-cdef-1234-567890abcdef",
		},
		{
			name:     "UUID with dashes",
			input:    "12345678-90ab-cdef-1234-567890abcdef",
			expected: "12345678-90ab-cdef-1234-567890abcdef",
		},
		{
			name:     "Full Notion URL",
			input:    "https://www.notion.so/workspace/My-Database-1234567890abcdef1234567890abcdef",
			expected: "12345678-90ab-cdef-1234-567890abcdef",
		},
		{
			name:     "Notion URL with query params",
			input:    "https://notion.so/My-Page-1234567890abcdef1234567890abcdef?v=abc",
			expected: "12345678-90ab-cdef-1234-567890abcdef",
		},
		{
			name:     "Uppercase hex",
			input:    "1234567890ABCDEF1234567890ABCDEF",
			expected: "12345678-90ab-cdef-1234-567890abcdef",
		},
		{
			name:     "With whitespace",
			input:    "  1234567890abcdef1234567890abcdef  ",
			expected: "12345678-90ab-cdef-1234-567890abcdef",
		},
		{
			name:    "Too short",
			input:   "1234567890abcdef",
			wantErr: true,
		},
		{
			name:    "Invalid URL",
			input:   "https://notion.so/invalid",
			wantErr: true,
		},
		{
			name:    "Empty",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeNotionID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NormalizeNotionID(%q) expected error, got %q", tt.input, result)
				}
				return
			}
			if err != nil {
				t.Errorf("NormalizeNotionID(%q) unexpected error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("NormalizeNotionID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
