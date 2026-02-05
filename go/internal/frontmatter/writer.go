package frontmatter

import (
	"fmt"
	"regexp"
	"strings"
)

// Build creates a markdown file with YAML frontmatter and body.
// Uses manual YAML serialization to match TypeScript output exactly.
func Build(frontmatter map[string]interface{}, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")

	for key, value := range frontmatter {
		sb.WriteString(formatYamlEntry(key, value))
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(body)

	return sb.String()
}

// BuildOrdered creates a markdown file with YAML frontmatter in a specific key order.
func BuildOrdered(frontmatter map[string]interface{}, keys []string, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")

	// Write keys in specified order
	written := make(map[string]bool)
	for _, key := range keys {
		if value, ok := frontmatter[key]; ok {
			sb.WriteString(formatYamlEntry(key, value))
			sb.WriteString("\n")
			written[key] = true
		}
	}

	// Write remaining keys
	for key, value := range frontmatter {
		if !written[key] {
			sb.WriteString(formatYamlEntry(key, value))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("---\n")
	sb.WriteString(body)

	return sb.String()
}

func formatYamlEntry(key string, value interface{}) string {
	safeKey := key
	if strings.Contains(key, ":") || strings.Contains(key, " ") {
		safeKey = fmt.Sprintf("%q", key)
	}

	switch v := value.(type) {
	case nil:
		return fmt.Sprintf("%s: null", safeKey)
	case bool:
		return fmt.Sprintf("%s: %t", safeKey, v)
	case int:
		return fmt.Sprintf("%s: %d", safeKey, v)
	case int64:
		return fmt.Sprintf("%s: %d", safeKey, v)
	case float64:
		// Format without unnecessary decimal places
		if v == float64(int64(v)) {
			return fmt.Sprintf("%s: %d", safeKey, int64(v))
		}
		return fmt.Sprintf("%s: %g", safeKey, v)
	case []interface{}:
		if len(v) == 0 {
			return fmt.Sprintf("%s: []", safeKey)
		}
		var items []string
		for _, item := range v {
			items = append(items, fmt.Sprintf("  - %s", yamlEscapeString(fmt.Sprintf("%v", item))))
		}
		return fmt.Sprintf("%s:\n%s", safeKey, strings.Join(items, "\n"))
	case []string:
		if len(v) == 0 {
			return fmt.Sprintf("%s: []", safeKey)
		}
		var items []string
		for _, item := range v {
			items = append(items, fmt.Sprintf("  - %s", yamlEscapeString(item)))
		}
		return fmt.Sprintf("%s:\n%s", safeKey, strings.Join(items, "\n"))
	default:
		return fmt.Sprintf("%s: %s", safeKey, yamlEscapeString(fmt.Sprintf("%v", v)))
	}
}

var digitOnlyRe = regexp.MustCompile(`^\d+$`)

func yamlEscapeString(str string) string {
	needsQuoting := strings.Contains(str, ":") ||
		strings.Contains(str, "#") ||
		strings.Contains(str, "'") ||
		strings.Contains(str, "\"") ||
		strings.Contains(str, "\n") ||
		strings.HasPrefix(str, " ") ||
		strings.HasPrefix(str, "-") ||
		strings.HasPrefix(str, "[") ||
		strings.HasPrefix(str, "{") ||
		str == "true" ||
		str == "false" ||
		str == "null" ||
		digitOnlyRe.MatchString(str)

	if needsQuoting {
		escaped := strings.ReplaceAll(str, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		return fmt.Sprintf("%q", escaped)
	}

	return str
}
