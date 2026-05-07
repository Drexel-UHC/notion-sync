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

// pushableFile is a folder entry the push pipeline will act on: a .md file
// linked to Notion (`notion-id` present) and not soft-deleted.
type pushableFile struct {
	path string
	fm   map[string]interface{}
}

// scanPushable is the single source of truth for "what counts as pushable" —
// both BuildPushQueue (preview) and PushDatabase (action) call it so the
// confirmation gate's preview-equals-action contract holds by construction,
// not by hand-keeping two filter loops in sync.
func scanPushable(folderPath string) ([]pushableFile, error) {
	dirEntries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}
	var files []pushableFile
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(folderPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			// Silent skip is correct here: the validation gate
			// (ValidatePushQueue) and the preview's scanLocalHalts both
			// classify unreadable files as ClassHaltUnreadable, so the user
			// sees them in the halt list. Erroring here would short-circuit
			// BuildPushQueue before scanLocalHalts runs and turn a nice halt
			// list into a raw "read foo.md: permission denied" error.
			// Under --force the gate is skipped — silent drop matches the
			// rest of yolo mode (strays, malformed YAML also silently skip).
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
		files = append(files, pushableFile{path: filePath, fm: fm})
	}
	return files, nil
}

// BuildPushQueue returns the phase-1 confirmation preview: the .md files
// PushDatabase would attempt to push, plus any halts detectable from disk
// alone (stray .md, malformed YAML). Used by the confirmation gate (DAG
// n12b) before any Notion API call.
//
// Errors if folderPath isn't a synced database so the user sees "not a
// sync folder" rather than a misleading "nothing to push". Surfacing
// LocalHalts here keeps the preview honest with the validation gate: if
// strays exist, the user sees them at consent time instead of confirming
// a queue and then getting halted on a file they were never shown.
func BuildPushQueue(folderPath string) (*PushPreview, error) {
	metadata, err := ReadDatabaseMetadata(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	if metadata == nil {
		return nil, fmt.Errorf("no %s found in %s. Use 'import' to import the database first", DatabaseMetadataFile, folderPath)
	}
	files, err := scanPushable(folderPath)
	if err != nil {
		return nil, err
	}
	queue := make([]string, 0, len(files))
	for _, f := range files {
		queue = append(queue, f.path)
	}
	localHalts, err := scanLocalHalts(folderPath)
	if err != nil {
		return nil, err
	}
	return &PushPreview{Queue: queue, LocalHalts: localHalts}, nil
}

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

	// In Notion's multi-data-source API, the property schema lives on the data source,
	// not the database. Newer _database.json files record the dataSourceId; fall back
	// to GetDatabase for legacy single-source metadata that predates the migration.
	var schema map[string]notion.DatabaseProperty
	if metadata.DataSourceID != "" {
		ds, err := opts.Client.GetDataSource(metadata.DataSourceID)
		if err != nil {
			return nil, fmt.Errorf("fetch data source schema: %w", err)
		}
		schema = ds.Properties
	} else {
		database, err := opts.Client.GetDatabase(metadata.DatabaseID)
		if err != nil {
			return nil, fmt.Errorf("fetch database schema: %w", err)
		}
		schema = database.Properties
	}
	if len(schema) == 0 {
		return nil, fmt.Errorf("database has no property schema; try re-importing the database first")
	}

	result := &PushResult{
		Title:      metadata.Title,
		FolderPath: opts.FolderPath,
	}

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhasePushScanning})
	}

	// Validation gate (DAG n21+n22). All-or-nothing: any halt-class file
	// across the whole folder aborts before any Notion write. --force skips
	// the gate entirely (existing escape hatch, matches phase-1 behavior).
	if !opts.Force {
		report, err := ValidatePushQueue(opts.Client, opts.FolderPath)
		if err != nil {
			return nil, err
		}
		if report.Halted {
			for _, f := range report.Files {
				if f.Class.IsHalt() {
					result.Halts = append(result.Halts, f)
				}
			}
			result.Halted = true
			// Total reflects everything classified, not just halts — gives
			// the CLI summary "halted: 3 of 9 inspected" instead of the
			// useless "Total: 0".
			result.Total = len(report.Files)
			if onProgress != nil {
				onProgress(ProgressPhase{Phase: PhaseComplete})
			}
			return result, nil
		}
	}

	files, err := scanPushable(opts.FolderPath)
	if err != nil {
		return nil, err
	}

	result.Total = len(files)

	for i, f := range files {
		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhasePushing, Current: i + 1, Total: result.Total, Title: metadata.Title})
		}

		notionID := f.fm["notion-id"].(string)
		localLastEdited, _ := f.fm["notion-last-edited"].(string)

		// TOCTOU defense: the validation gate already covered the !Force
		// common case, but Notion could be edited in the window between
		// the gate's GetPage and this one. The gate makes per-row halts
		// nearly impossible in practice; this catches the rare race so
		// we never overwrite a freshly-edited row.
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

		properties, validationErrs := buildPropertyPayload(f.fm, schema)
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

		// UpdatePage's response echoes last_edited_time quantized to whole
		// minutes, but Notion's stored value (returned by GetPage / QueryDataSource)
		// is precise. Re-fetch so the local frontmatter holds the precise value
		// and the next refresh doesn't see the page as stale. See issue #57.
		newLastEdited := ""
		refetched, refetchErr := opts.Client.GetPage(notionID)
		if refetchErr == nil && refetched != nil {
			newLastEdited = refetched.LastEditedTime
		} else {
			if refetchErr != nil {
				// Non-fatal: push succeeded; we just couldn't refetch the precise
				// timestamp. Surface it so silent failures don't quietly reintroduce
				// the quantized-timestamp bug this code exists to avoid.
				result.Errors = append(result.Errors, fmt.Sprintf("%s: refetch precise timestamp: %v", filepath.Base(f.path), refetchErr))
			}
			if updated != nil && updated.LastEditedTime != "" {
				newLastEdited = updated.LastEditedTime
			} else if notionPage != nil {
				newLastEdited = notionPage.LastEditedTime
			}
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
	"notion-id": true, "notion-url": true,
	"notion-last-edited": true, "notion-database-id": true,
	"notion-deleted": true, "notion-last-pushed": true,
}

var pushSkipTypes = map[string]bool{
	"people":       true,
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
		if prop.Type == "rich_text" || prop.Type == "title" {
			if s := coerceString(val); len(s) > 2000 {
				errs = append(errs, fmt.Sprintf("%q exceeds Notion's 2000-char limit for %s (got %d chars)", key, prop.Type, len(s)))
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
	case "title", "rich_text":
		s := coerceString(val)
		return map[string]interface{}{
			propType: []interface{}{
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
				"start": stripMidnightUTC(strings.TrimSpace(parts[0])),
				"end":   stripMidnightUTC(strings.TrimSpace(parts[1])),
			},
		}
	}
	return map[string]interface{}{
		"date": map[string]interface{}{"start": stripMidnightUTC(s)},
	}
}

// stripMidnightUTC demotes "YYYY-MM-DDT00:00:00Z" back to "YYYY-MM-DD".
// Workaround for yaml.v3 + frontmatter.Parse collapsing date-only scalars
// into RFC3339 datetimes — without this, every date-only property gets
// promoted to a UTC datetime on Notion (is_datetime flips false→true) on
// every push. See .context/features/push/backlog/date-only-roundtrip.md
// for the proper fix (yaml.Node parsing to preserve original tokens).
func stripMidnightUTC(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	if t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0 || t.Nanosecond() != 0 {
		return s
	}
	_, off := t.Zone()
	if off != 0 {
		return s
	}
	return t.Format("2006-01-02")
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
