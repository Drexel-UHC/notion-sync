package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ran-codes/notion-sync/internal/frontmatter"
	"github.com/ran-codes/notion-sync/internal/notion"
)

// PushDatabase pushes local frontmatter property changes back to Notion.
// Only page properties are updated; page body content is never modified.
func PushDatabase(opts PushOptions, onProgress ProgressCallback) (*PushResult, error) {
	metadata, err := ReadDatabaseMetadata(opts.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	if metadata == nil {
		return nil, fmt.Errorf("no %s found in %s. Use 'import' to import the database first", DatabaseMetadataFile, opts.FolderPath)
	}

	database, err := opts.Client.GetDatabase(metadata.DatabaseID)
	if err != nil {
		return nil, fmt.Errorf("fetch database schema: %w", err)
	}
	if len(database.Properties) == 0 {
		return nil, fmt.Errorf("database has no property schema; try re-importing the database first")
	}

	result := &PushResult{
		Title:      metadata.Title,
		FolderPath: opts.FolderPath,
	}

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhasePushScanning})
	}

	type fileEntry struct {
		path string
		fm   map[string]interface{}
	}

	dirEntries, err := os.ReadDir(opts.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}

	var files []fileEntry
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(opts.FolderPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		fm, err := frontmatter.Parse(string(content))
		if err != nil || fm == nil {
			continue
		}
		if _, ok := fm["notion-id"].(string); !ok {
			continue
		}
		if deleted, ok := fm["notion-deleted"].(bool); ok && deleted {
			continue
		}
		files = append(files, fileEntry{filePath, fm})
	}

	result.Total = len(files)

	for i, f := range files {
		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhasePushing, Current: i + 1, Total: result.Total, Title: metadata.Title})
		}

		notionID := f.fm["notion-id"].(string)
		localLastEdited, _ := f.fm["notion-last-edited"].(string)

		// Conflict check: compare local last-edited timestamp with Notion's current value.
		var notionPage *notion.Page
		if !opts.Force {
			page, err := opts.Client.GetPage(notionID)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filepath.Base(f.path), err))
				continue
			}
			if !timestampsEqual(localLastEdited, page.LastEditedTime) {
				result.Conflicts++
				result.ConflictFiles = append(result.ConflictFiles, filepath.Base(f.path))
				continue
			}
			notionPage = page
		}

		properties, validationErrs := buildPropertyPayload(f.fm, database.Properties)
		if len(validationErrs) > 0 {
			result.Failed++
			for _, e := range validationErrs {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", filepath.Base(f.path), e))
			}
			continue
		}
		if len(properties) == 0 {
			result.Skipped++
			continue
		}

		if opts.DryRun {
			result.Pushed++
			continue
		}

		updated, err := opts.Client.UpdatePage(notionID, properties)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filepath.Base(f.path), err))
			continue
		}

		// Use the returned last_edited_time if available, otherwise fall back to the
		// value we already fetched during conflict check.
		newLastEdited := ""
		if updated != nil && updated.LastEditedTime != "" {
			newLastEdited = updated.LastEditedTime
		} else if notionPage != nil {
			newLastEdited = notionPage.LastEditedTime
		}

		pushedAt := time.Now().UTC().Format(time.RFC3339)
		if err := updateAfterPush(f.path, newLastEdited, pushedAt); err != nil {
			// Non-fatal: push succeeded, just couldn't update local timestamps.
			result.Errors = append(result.Errors, fmt.Sprintf("%s: update timestamps: %v", filepath.Base(f.path), err))
		}

		result.Pushed++
	}

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseComplete})
	}

	return result, nil
}

// buildPropertyPayload constructs the Notion API property update payload from frontmatter.
// Uses the database schema to determine property types; skips read-only / Notion-native properties.
var pushNotionKeys = map[string]bool{
	"notion-id": true, "notion-url": true, "notion-frozen-at": true,
	"notion-last-edited": true, "notion-database-id": true,
	"notion-deleted": true, "notion-last-pushed": true,
}

var pushSkipTypes = map[string]bool{
	"title": true, "people": true,
	"created_time": true, "last_edited_time": true,
	"created_by": true, "last_edited_by": true,
	"formula": true, "rollup": true, "button": true,
	"unique_id": true, "verification": true, "files": true,
}

func buildPropertyPayload(fm map[string]interface{}, schema map[string]notion.DatabaseProperty) (map[string]interface{}, []string) {
	result := make(map[string]interface{})
	var errs []string
	for key, val := range fm {
		if pushNotionKeys[key] {
			continue
		}
		prop, ok := schema[key]
		if !ok {
			continue
		}
		if pushSkipTypes[prop.Type] {
			continue
		}
		if prop.Type == "rich_text" {
			if s := coerceString(val); len(s) > 2000 {
				errs = append(errs, fmt.Sprintf("%q exceeds Notion's 2000-char limit for rich_text (got %d chars)", key, len(s)))
				continue
			}
		}
		payload := buildPropertyValue(prop.Type, val)
		if payload != nil {
			result[key] = payload
		}
	}
	return result, errs
}

func buildPropertyValue(propType string, val interface{}) interface{} {
	switch propType {
	case "rich_text":
		s := coerceString(val)
		return map[string]interface{}{
			"rich_text": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": map[string]interface{}{"content": s},
				},
			},
		}

	case "number":
		if val == nil {
			return map[string]interface{}{"number": nil}
		}
		switch v := val.(type) {
		case float64:
			return map[string]interface{}{"number": v}
		case int:
			return map[string]interface{}{"number": float64(v)}
		}
		return nil

	case "select":
		if val == nil {
			return map[string]interface{}{"select": nil}
		}
		s := coerceString(val)
		if s == "" {
			return map[string]interface{}{"select": nil}
		}
		return map[string]interface{}{"select": map[string]interface{}{"name": s}}

	case "multi_select":
		items := coerceStringSlice(val)
		opts := make([]interface{}, 0, len(items))
		for _, name := range items {
			opts = append(opts, map[string]interface{}{"name": name})
		}
		return map[string]interface{}{"multi_select": opts}

	case "status":
		if val == nil {
			return nil
		}
		s := coerceString(val)
		if s == "" {
			return nil
		}
		return map[string]interface{}{"status": map[string]interface{}{"name": s}}

	case "date":
		if val == nil {
			return map[string]interface{}{"date": nil}
		}
		return parseDatePayload(coerceString(val))

	case "checkbox":
		if b, ok := val.(bool); ok {
			return map[string]interface{}{"checkbox": b}
		}
		return nil

	case "url":
		if val == nil {
			return map[string]interface{}{"url": nil}
		}
		return map[string]interface{}{"url": coerceString(val)}

	case "email":
		if val == nil {
			return map[string]interface{}{"email": nil}
		}
		return map[string]interface{}{"email": coerceString(val)}

	case "phone_number":
		if val == nil {
			return map[string]interface{}{"phone_number": nil}
		}
		return map[string]interface{}{"phone_number": coerceString(val)}

	case "relation":
		ids := coerceStringSlice(val)
		rels := make([]interface{}, 0, len(ids))
		for _, id := range ids {
			rels = append(rels, map[string]interface{}{"id": id})
		}
		return map[string]interface{}{"relation": rels}
	}
	return nil
}

func parseDatePayload(s string) interface{} {
	if s == "" {
		return map[string]interface{}{"date": nil}
	}
	if strings.Contains(s, " → ") {
		parts := strings.SplitN(s, " → ", 2)
		return map[string]interface{}{
			"date": map[string]interface{}{
				"start": strings.TrimSpace(parts[0]),
				"end":   strings.TrimSpace(parts[1]),
			},
		}
	}
	return map[string]interface{}{
		"date": map[string]interface{}{"start": s},
	}
}

func coerceString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf("%v", val)
}

func coerceStringSlice(val interface{}) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, coerceString(item))
		}
		return result
	case []string:
		return v
	}
	return nil
}

// updateAfterPush updates notion-last-edited and adds notion-last-pushed in the file's frontmatter.
func updateAfterPush(filePath, newLastEdited, pushedAt string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	s := strings.ReplaceAll(string(content), "\r\n", "\n")

	// Update notion-last-edited if we have a new value from Notion.
	// Search for "\nnotion-last-edited:" (newline-anchored) to avoid matching
	// the substring inside a property value on a different line.
	if newLastEdited != "" {
		if idx := strings.Index(s, "\nnotion-last-edited:"); idx != -1 {
			keyStart := idx + 1 // skip the leading \n
			end := strings.Index(s[keyStart:], "\n")
			if end != -1 {
				s = s[:keyStart] + "notion-last-edited: " + newLastEdited + s[keyStart+end:]
			}
		}
	}

	pushedLine := "notion-last-pushed: " + pushedAt

	// Update existing notion-last-pushed.
	if idx := strings.Index(s, "\nnotion-last-pushed:"); idx != -1 {
		keyStart := idx + 1
		end := strings.Index(s[keyStart:], "\n")
		if end != -1 {
			s = s[:keyStart] + pushedLine + s[keyStart+end:]
		}
		return os.WriteFile(filePath, []byte(s), 0644)
	}

	// Insert after notion-last-edited.
	if idx := strings.Index(s, "\nnotion-last-edited:"); idx != -1 {
		keyStart := idx + 1
		end := strings.Index(s[keyStart:], "\n")
		if end != -1 {
			insertAt := keyStart + end
			s = s[:insertAt] + "\n" + pushedLine + s[insertAt:]
			return os.WriteFile(filePath, []byte(s), 0644)
		}
	}

	// Fallback: insert before closing ---.
	if strings.HasPrefix(s, "---\n") {
		endIdx := strings.Index(s[4:], "\n---")
		if endIdx != -1 {
			insertAt := 4 + endIdx
			s = s[:insertAt] + "\n" + pushedLine + s[insertAt:]
			return os.WriteFile(filePath, []byte(s), 0644)
		}
	}

	return nil
}
