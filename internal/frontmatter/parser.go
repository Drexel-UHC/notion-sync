// Package frontmatter handles parsing and writing YAML frontmatter in Markdown files.
package frontmatter

import (
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Parse extracts YAML frontmatter from markdown content.
// Returns nil if no frontmatter is found.
func Parse(content string) (map[string]interface{}, error) {
	if !strings.HasPrefix(content, "---\n") {
		return nil, nil
	}

	endIdx := strings.Index(content[4:], "\n---")
	if endIdx == -1 {
		return nil, nil
	}

	yamlStr := content[4 : 4+endIdx]

	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &result); err != nil {
		return nil, err
	}

	normalizeMapValues(result)

	return result, nil
}

// normalizeMapValues converts any time.Time values back to RFC3339 strings.
// yaml.v3 automatically parses ISO 8601 strings into time.Time, which breaks
// downstream .(string) type assertions.
func normalizeMapValues(m map[string]interface{}) {
	for k, v := range m {
		switch val := v.(type) {
		case time.Time:
			m[k] = val.Format(time.RFC3339)
		case []interface{}:
			for i, item := range val {
				if t, ok := item.(time.Time); ok {
					val[i] = t.Format(time.RFC3339)
				}
			}
		}
	}
}

// GetBody extracts the body content after frontmatter.
func GetBody(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}

	endIdx := strings.Index(content[4:], "\n---")
	if endIdx == -1 {
		return content
	}

	// Skip past "---\n" + yaml + "\n---\n"
	bodyStart := 4 + endIdx + 4
	if bodyStart >= len(content) {
		return ""
	}

	// Skip potential leading newline
	body := content[bodyStart:]
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}

	return body
}
