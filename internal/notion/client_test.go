package notion

import "testing"

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  *ErrorResponse
		want bool
	}{
		{
			// Auto-detect: GET /databases/{page-id} returns 400 validation_error.
			// runImport relies on this to fall back to page import.
			name: "400 validation_error 'is a page, not a database'",
			err: &ErrorResponse{
				Status:  400,
				Code:    "validation_error",
				Message: "Provided database_id 31357008-e885-80c3-90f4-d148f0854bba is a page, not a database. Use the pages API instead, or pass the ID of the database itself.",
			},
			want: true,
		},
		{
			name: "404 object_not_found",
			err:  &ErrorResponse{Status: 404, Code: "object_not_found", Message: "Could not find database"},
			want: true,
		},
		{
			name: "401 API token is invalid (page ID on database endpoint)",
			err:  &ErrorResponse{Status: 401, Code: "unauthorized", Message: "API token is invalid"},
			want: true,
		},
		{
			name: "400 validation_error unrelated message",
			err:  &ErrorResponse{Status: 400, Code: "validation_error", Message: "body failed validation: body.properties is not a recognized property"},
			want: false,
		},
		{
			name: "401 unrelated unauthorized",
			err:  &ErrorResponse{Status: 401, Code: "unauthorized", Message: "Insufficient permissions"},
			want: false,
		},
		{
			name: "500 internal error",
			err:  &ErrorResponse{Status: 500, Code: "internal_server_error", Message: "Something went wrong"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFoundError(tt.err); got != tt.want {
				t.Errorf("IsNotFoundError(%+v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

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
