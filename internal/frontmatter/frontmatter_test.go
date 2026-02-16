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
		{
			name: "quoted ISO 8601 timestamp with milliseconds returns string",
			content: `---
notion-last-edited: "2025-01-15T10:30:00.000Z"
---
Body`,
			expected: map[string]interface{}{
				// Quoted strings stay as-is in yaml.v3 (already a string, not time.Time).
				// timestampsEqual() handles .000Z vs Z comparison.
				"notion-last-edited": "2025-01-15T10:30:00.000Z",
			},
		},
		{
			name: "unquoted ISO 8601 timestamp returns string",
			content: `---
notion-last-edited: 2025-01-15T10:30:00Z
---
Body`,
			expected: map[string]interface{}{
				"notion-last-edited": "2025-01-15T10:30:00Z",
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

func TestBuildAndParseRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  interface{}
		want interface{} // expected after Parse (nil means same as val)
	}{
		{"negative number", "score", float64(-42.5), float64(-42.5)},
		{"zero number", "count", float64(0), 0},                       // yaml parses int 0 as int
		{"large number", "big", float64(999999.99), float64(999999.99)},
		{"string with colon", "label", "key: value", "key: value"},
		{"unicode", "name", "Très résumé 日本語", "Très résumé 日本語"},
		{"empty array", "tags", []interface{}{}, []interface{}{}},
		{"nil value", "empty", nil, nil},
		{"boolean-like string true", "status", "true", "true"},
		{"boolean-like string false", "flag", "false", "false"},
		{"boolean-like string null", "nothing", "null", "null"},
		{"timestamp string", "edited", "2025-01-15T10:30:00Z", "2025-01-15T10:30:00Z"},
		{"negative integer-like float", "neg", float64(-7), -7},
		{"string starting with dash", "note", "-starts-here", "-starts-here"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := map[string]interface{}{tt.key: tt.val}
			content := Build(fm, "body")

			parsed, err := Parse(content)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			want := tt.want
			if want == nil {
				want = tt.val
			}
			got := parsed[tt.key]

			// Handle nil comparison
			if want == nil {
				if got != nil {
					t.Errorf("key %q: got %v (%T), want nil", tt.key, got, got)
				}
				return
			}

			// Handle empty array
			if wantArr, ok := want.([]interface{}); ok {
				gotArr, ok := got.([]interface{})
				if !ok {
					// yaml.v3 returns nil for empty arrays parsed from "[]"
					if len(wantArr) == 0 && got == nil {
						return
					}
					t.Errorf("key %q: got %T, want []interface{}", tt.key, got)
					return
				}
				if len(gotArr) != len(wantArr) {
					t.Errorf("key %q: len = %d, want %d", tt.key, len(gotArr), len(wantArr))
				}
				return
			}

			if got != want {
				t.Errorf("key %q: got %v (%T), want %v (%T)", tt.key, got, got, want, want)
			}
		})
	}
}

func TestFormatYamlEntry_BoundaryNumbers(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
	}{
		{"negative float", float64(-42.5), "n: -42.5"},
		{"zero", float64(0), "n: 0"},
		{"large float", float64(999999.99), "n: 999999.99"},
		{"integer-like float", float64(100), "n: 100"},
		{"negative integer-like", float64(-7), "n: -7"},
		{"very small", float64(0.001), "n: 0.001"},
		{"int type", 42, "n: 42"},
		{"int64 type", int64(9999999), "n: 9999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatYamlEntry("n", tt.val)
			if got != tt.want {
				t.Errorf("formatYamlEntry(\"n\", %v) = %q, want %q", tt.val, got, tt.want)
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
		{"has\nnewline", `"has\nnewline"`},
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
