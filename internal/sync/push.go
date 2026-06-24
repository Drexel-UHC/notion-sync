package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// readyQueue extracts the ClassReady rows from a classifier report (#80) —
// the files PushDatabase will attempt to push. Deriving the queue from the
// same report that produces the halts is what makes the preview-equals-action
// contract hold by construction rather than by hand-keeping parallel filter
// loops in sync.
func readyQueue(report *ValidationReport) []FileClassification {
	queue := make([]FileClassification, 0, len(report.Files))
	for _, f := range report.Files {
		if f.Class == ClassReady {
			queue = append(queue, f)
		}
	}
	return queue
}

// BuildPushQueue returns the phase-1 confirmation preview: the .md files
// PushDatabase would attempt to push, plus any halts detectable from disk
// alone (stray .md, malformed YAML, unreadable). Used by the confirmation
// gate (DAG n12b) before any Notion API call.
//
// Errors if folderPath isn't a synced database so the user sees "not a
// sync folder" rather than a misleading "nothing to push". Both Queue and
// LocalHalts come from one network-free classifyFolder walk: the queue is
// its ClassReady rows, the halts are its locally-detectable halt rows. That
// keeps the preview honest with the validation gate — if strays exist, the
// user sees them at consent time instead of confirming a queue and then
// getting halted on a file they were never shown.
func BuildPushQueue(folderPath string) (*PushPreview, error) {
	metadata, err := ReadDatabaseMetadata(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	if metadata == nil {
		return nil, fmt.Errorf("no %s found in %s. Use 'import' to import the database first", DatabaseMetadataFile, folderPath)
	}
	// nil client + nil schema → network-free local classification. Conflict /
	// Unreachable / InvalidOption can't surface here (they need a GetPage or the
	// fetched schema); they wait for the gate.
	report, err := classifyFolder(folderPath, nil, nil, false)
	if err != nil {
		return nil, err
	}
	preview := &PushPreview{Queue: make([]string, 0, len(report.Files))}
	for _, f := range report.Files {
		switch {
		case f.Class == ClassReady:
			preview.Queue = append(preview.Queue, f.Path)
		case f.Class.isLocalHalt():
			preview.LocalHalts = append(preview.LocalHalts, f)
		}
	}
	return preview, nil
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

	// Single classifier walk (#80). Under the gate (!Force) the client
	// resolves conflicts; under --force we run network-free so the gate is
	// skipped entirely (existing escape hatch). Both the halt list and the
	// push queue derive from this one report — no second folder walk.
	gateClient := opts.Client
	gateSchema := schema
	if opts.Force {
		gateClient = nil
		gateSchema = nil
	}
	report, err := classifyFolder(opts.FolderPath, gateClient, gateSchema, opts.AllowNewOptions)
	if err != nil {
		return nil, err
	}

	// Validation gate (DAG n21+n22). All-or-nothing: any halt-class file
	// across the whole folder aborts before any Notion write. --force ran the
	// walk network-free, so report.Halted is false and this branch is skipped.
	if !opts.Force && report.Halted {
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

	files := readyQueue(report)

	result.Total = len(files)

	for i, f := range files {
		if onProgress != nil {
			onProgress(ProgressPhase{Phase: PhasePushing, Current: i + 1, Total: result.Total, Title: metadata.Title})
		}

		notionID := f.NotionID
		localLastEdited, _ := f.fm["notion-last-edited"].(string)

		// Per-cell diff (DAG n31/n32): compare local frontmatter to the snapshot
		// the gate already stashed. Empty diff → nothing changed → skippedNoOp
		// (n32a), and we skip the TOCTOU re-fetch entirely — only rows that
		// actually change pay for the second GetPage (decision #2). --force has no
		// stash and deliberately overwrites, so it bypasses the diff.
		//
		// changedFields stays nil under --force, which means "send every pushable
		// field" — the deliberate overwrite-blind path. On the gate path it's the
		// re-diff against the fresh re-fetch, so the push sends ONLY what changed.
		var notionPage *notion.Page
		var changedFields []string
		if !opts.Force {
			if len(diffRow(f.fm, f.page, schema)) == 0 {
				result.SkippedNoOp++
				continue
			}

			// TOCTOU defense (n32b): the row changed locally, so re-fetch fresh
			// right before writing. Notion has no conditional write — a moved
			// timestamp here means it was edited since the gate read it (n32c),
			// so skip to avoid clobbering rather than overwrite blind.
			page, err := opts.Client.GetPage(notionID)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filepath.Base(f.Path), err))
				continue
			}
			if !timestampsEqual(localLastEdited, page.LastEditedTime) {
				result.Conflicts++
				result.ConflictFiles = append(result.ConflictFiles, filepath.Base(f.Path))
				continue
			}
			notionPage = page

			// Re-diff against the fresh re-fetch (decision #2): this is the
			// authoritative changed-field set the push sends (n33). A row that
			// raced into equality between the gate read and now is a no-op.
			changedFields = diffRow(f.fm, page, schema)
			if len(changedFields) == 0 {
				result.SkippedNoOp++
				continue
			}
		}

		// n33: send ONLY the changed fields. Under --force changedFields is nil,
		// which buildPropertyPayloadFor treats as "all pushable fields".
		properties, validationErrs := buildPropertyPayloadFor(f.fm, schema, changedFields)
		if len(validationErrs) > 0 {
			result.Failed++
			for _, e := range validationErrs {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", filepath.Base(f.Path), e))
			}
			continue
		}
		// With the payload narrowed to changedFields, an empty payload means the
		// only changed field can't be expressed as a write (e.g. a buildPropertyValue
		// that returns nil) — count it Skipped rather than push an empty PATCH.
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
			// n34h: 401/403 is run-wide — the credential, not the row, failed.
			// Halt now so the remaining rows skip the API entirely (one clean
			// error instead of N identical ones). Rows already pushed stay pushed.
			if notion.IsAuthError(err) {
				result.AuthHalted = true
				result.AuthError = fmt.Sprintf("authentication failed (%v) — check the API key has write access to this database, then re-run", err)
				break
			}
			// n34c: per-row 4xx / exhausted-transient — loud-fail, keep going.
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filepath.Base(f.Path), err))
			continue
		}

		// Read-back (DAG n34d + n35a): re-fetch the row once. The same fetch does
		// two jobs — verify the write stored what we sent, and read Notion's
		// precise last_edited_time. UpdatePage's response echoes last_edited_time
		// quantized to whole minutes, but the stored value is precise (issue #57).
		refetched, refetchErr := opts.Client.GetPage(notionID)

		// n34d/n34e: verify each changed field round-tripped. A mismatch is a LOUD
		// per-row failure — the row is NOT restamped, so the next run re-attempts
		// rather than trusting a write that didn't take. Skipped under --force (no
		// changedFields; deliberate blind overwrite) and when the refetch failed
		// (can't verify — surfaced as the non-fatal timestamp warning below).
		//
		// Assumes Notion is read-after-write consistent for our token (it is): a
		// stale read here would spuriously fail the row, but the write is
		// idempotent so the next run re-pushes and re-verifies cleanly.
		if !opts.Force && refetchErr == nil && refetched != nil {
			if mismatches := verifyStoredFields(f.fm, refetched, schema, changedFields); len(mismatches) > 0 {
				result.Failed++
				parts := make([]string, 0, len(mismatches))
				for _, m := range mismatches {
					parts = append(parts, fmt.Sprintf("%s (sent %q, stored %q)", m.Field, m.Sent, m.Stored))
				}
				result.Errors = append(result.Errors, fmt.Sprintf("%s: Notion stored %s differently than sent — not restamped; re-check and re-push", filepath.Base(f.Path), strings.Join(parts, ", ")))
				continue
			}
		}

		newLastEdited := ""
		if refetchErr == nil && refetched != nil {
			newLastEdited = refetched.LastEditedTime
		} else {
			if refetchErr != nil {
				// Non-fatal: push succeeded; we just couldn't refetch the precise
				// timestamp. Surface it so silent failures don't quietly reintroduce
				// the quantized-timestamp bug this code exists to avoid.
				result.Errors = append(result.Errors, fmt.Sprintf("%s: refetch precise timestamp: %v", filepath.Base(f.Path), refetchErr))
			}
			if updated != nil && updated.LastEditedTime != "" {
				newLastEdited = updated.LastEditedTime
			} else if notionPage != nil {
				newLastEdited = notionPage.LastEditedTime
			}
		}

		pushedAt := time.Now().UTC().Format(time.RFC3339)
		if err := updateAfterPush(f.Path, newLastEdited, pushedAt); err != nil {
			// Non-fatal: push succeeded, just couldn't update local timestamps.
			result.Errors = append(result.Errors, fmt.Sprintf("%s: update timestamps: %v", filepath.Base(f.Path), err))
		}

		result.Pushed++
	}

	if onProgress != nil {
		onProgress(ProgressPhase{Phase: PhaseComplete})
	}

	return result, nil
}

// diffRow returns the schema-backed property keys whose local frontmatter value
// differs from the gate's stashed Notion snapshot (DAG n31). Pure, no I/O.
//
// It mirrors buildPropertyPayload's iteration (same notion-key / read-only-type
// skips) so the diff can never flag a field the push wouldn't actually send.
// rich_text is additionally skipped — the deliberate "3a skip" (issue #55):
// pushing rich_text as literal plain text corrupts formatting, so until #95's
// parser is wired a rich-text-only edit must not count as a change. The snapshot
// is decoded with mapPropertiesToFrontmatter so both sides are compared in the
// same frontmatter representation they were imported in.
func diffRow(fm map[string]interface{}, snapshot *notion.Page, schema map[string]notion.DatabaseProperty) []string {
	remote := map[string]interface{}{}
	if snapshot != nil {
		mapPropertiesToFrontmatter(snapshot.Properties, remote, false)
	}
	var changed []string
	for key, localVal := range fm {
		prop, ok := pushableField(key, schema)
		if !ok {
			continue
		}
		// The deliberate "3a skip": rich_text is excluded from the diff (but not
		// from buildPropertyPayload's send) until #95's parser lands, so a
		// rich_text-only edit is a no-op instead of a formatting-corrupting push.
		if prop.Type == "rich_text" {
			continue
		}
		if !valuesEqual(prop.Type, localVal, remote[key]) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}

// fieldMismatch is one read-back discrepancy: a changed field whose stored value
// differs from what the push sent (DAG n34e). Sent/Stored carry the human-readable
// values so the failure line can show both, not just the field name.
type fieldMismatch struct {
	Field  string
	Sent   string
	Stored string
}

// verifyStoredFields re-decodes the row Notion stored after a push and returns a
// mismatch for each changed field whose stored value does not match what we sent
// (DAG n34d). A non-empty result is a read-back mismatch (n34e). It compares only
// the fields the push actually sent (changedFields) and, like diffRow, skips
// rich_text — rich_text is never sent (the 3a skip), so it can be neither
// verified nor blamed. mapPropertiesToFrontmatter + valuesEqual decode and
// compare both sides in the same representation the diff used, so verify can't
// disagree with diff.
func verifyStoredFields(fm map[string]interface{}, stored *notion.Page, schema map[string]notion.DatabaseProperty, changedFields []string) []fieldMismatch {
	remote := map[string]interface{}{}
	if stored != nil {
		mapPropertiesToFrontmatter(stored.Properties, remote, false)
	}
	var mismatches []fieldMismatch
	for _, key := range changedFields {
		prop, ok := pushableField(key, schema)
		if !ok {
			continue
		}
		if prop.Type == "rich_text" {
			continue
		}
		if !valuesEqual(prop.Type, fm[key], remote[key]) {
			mismatches = append(mismatches, fieldMismatch{
				Field:  key,
				Sent:   fmt.Sprintf("%v", fm[key]),
				Stored: fmt.Sprintf("%v", remote[key]),
			})
		}
	}
	sort.Slice(mismatches, func(i, j int) bool { return mismatches[i].Field < mismatches[j].Field })
	return mismatches
}

// pushableField reports whether a frontmatter key maps to a schema property the
// push may write, returning that property. It centralizes the skip rules
// (notion-managed keys, keys absent from the schema, read-only/native types) so
// diffRow and buildPropertyPayload can never drift on what counts as pushable —
// the diff must never flag a field the push wouldn't actually send.
func pushableField(key string, schema map[string]notion.DatabaseProperty) (notion.DatabaseProperty, bool) {
	if pushNotionKeys[key] {
		return notion.DatabaseProperty{}, false
	}
	prop, ok := schema[key]
	if !ok {
		return notion.DatabaseProperty{}, false
	}
	if pushSkipTypes[prop.Type] {
		return notion.DatabaseProperty{}, false
	}
	return prop, true
}

// valuesEqual reports whether a local frontmatter value and the decoded Notion
// snapshot value represent the same stored property, using type-aware equality.
func valuesEqual(propType string, local, remote interface{}) bool {
	switch propType {
	case "title", "select", "status", "url", "email", "phone_number":
		// Scalar strings: a cleared value is nil from Notion's decode but may be
		// "" locally. coerceString folds nil→"" so they compare equal.
		return coerceString(local) == coerceString(remote)
	case "multi_select":
		// Unordered set in Notion — a reorder is not a change.
		return stringSetsEqual(coerceStringSlice(local), coerceStringSlice(remote))
	case "date":
		// Normalize both sides the same way the push would send (midnight-UTC
		// demoted to date-only) so yaml's RFC3339 promotion isn't a false diff.
		return normalizeDate(coerceString(local)) == normalizeDate(coerceString(remote))
	case "number":
		// yaml yields int for whole numbers; Notion's decode yields float64.
		// Compare numerically so the Go type difference isn't a false diff.
		return numbersEqual(local, remote)
	}
	return reflect.DeepEqual(local, remote)
}

// numbersEqual compares two number values across Go types (yaml's int vs
// Notion's float64). A nil on exactly one side is a real change (cleared vs set);
// nil on both is equal.
func numbersEqual(a, b interface{}) bool {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if !aok || !bok {
		return aok == bok
	}
	return af == bf
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// normalizeDate demotes midnight-UTC datetimes to date-only on each end of a
// (possibly ranged) date string, mirroring parseDatePayload so the diff compares
// values in the exact form the push would send them.
func normalizeDate(s string) string {
	if s == "" {
		return ""
	}
	if strings.Contains(s, " → ") {
		parts := strings.SplitN(s, " → ", 2)
		return stripMidnightUTC(strings.TrimSpace(parts[0])) + " → " + stripMidnightUTC(strings.TrimSpace(parts[1]))
	}
	return stripMidnightUTC(s)
}

// stringSetsEqual reports whether two string slices contain the same members
// regardless of order. Used for multi_select, which Notion stores unordered.
func stringSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
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
	return buildPropertyPayloadFor(fm, schema, nil)
}

// buildPropertyPayloadFor is buildPropertyPayload restricted to a set of keys.
// A nil `only` means "every pushable field" (the --force overwrite-blind path);
// a non-nil `only` restricts the payload to exactly those keys — the changed-
// field set from the diff (DAG n33). Restricting to changed fields also means a
// rich_text field that the diff skips never reaches the payload on a mixed edit,
// so a scalar change can't drag a formatting-corrupting rich_text push along.
func buildPropertyPayloadFor(fm map[string]interface{}, schema map[string]notion.DatabaseProperty, only []string) (map[string]interface{}, []string) {
	var onlySet map[string]bool
	if only != nil {
		onlySet = make(map[string]bool, len(only))
		for _, k := range only {
			onlySet[k] = true
		}
	}
	result := make(map[string]interface{})
	var errs []string
	for key, val := range fm {
		if onlySet != nil && !onlySet[key] {
			continue
		}
		prop, ok := pushableField(key, schema)
		if !ok {
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
