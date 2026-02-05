package frontmatter

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]interface{}
	}{
		{
			name: "basic frontmatter",
			content: `---
title: Hello
count: 5
---
Body content`,
			expected: map[string]interface{}{
				"title": "Hello",
				"count": 5,
			},
		},
		{
			name:     "no frontmatter",
			content:  "Just regular content",
			expected: nil,
		},
		{
			name: "empty frontmatter",
			content: `---
---
Body`,
			expected: map[string]interface{}{},
		},
		{
			name: "with arrays",
			content: `---
tags:
  - one
  - two
---
Body`,
			expected: map[string]interface{}{
				"tags": []interface{}{"one", "two"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.content)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			for k, v := range tt.expected {
				if !reflect.DeepEqual(result[k], v) {
					t.Errorf("key %q: got %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

func TestGetBody(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "with frontmatter",
			content: `---
title: Hello
---
Body content`,
			expected: "Body content",
		},
		{
			name:     "no frontmatter",
			content:  "Just content",
			expected: "Just content",
		},
		{
			name: "empty body",
			content: `---
title: Hello
---
`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBody(tt.content)
			if result != tt.expected {
				t.Errorf("GetBody = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestYamlEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello: world", `"hello: world"`},
		{"has#hash", `"has#hash"`},
		{"true", `"true"`},
		{"false", `"false"`},
		{"null", `"null"`},
		{"123", `"123"`},
		{"-starts-with-dash", `"-starts-with-dash"`},
		{"has\nnewline", `"has\\nnewline"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := yamlEscapeString(tt.input)
			if result != tt.expected {
				t.Errorf("yamlEscapeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
