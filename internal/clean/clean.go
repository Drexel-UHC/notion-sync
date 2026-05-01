// Package clean performs in-place cleanups on an already-imported notion-sync
// workspace, without making any Notion API calls. It exists to absorb one-time
// migrations after upgrading the binary so routine refreshes produce focused
// diffs.
//
// Cleanups applied:
//   - Strips Notion's S3 pre-signed query strings (X-Amz-Signature, etc.) from
//     file URLs in .md content. The query string rotates hourly while the path
//     is stable, so leaving them in produced large noise diffs on every refresh.
//   - Removes the deprecated `notion-frozen-at` line from YAML frontmatter. The
//     field used to be written on every freeze, which churned every entry's
//     diff even when content was byte-identical.
//   - Ensures .md and .json files end with a trailing newline.
//
// For each folder it modifies, the corresponding _database.json or _page.json
// is re-written so the syncVersion field reflects the binary that performed
// the cleanup — giving the workspace an audit trail of which notion-sync
// version last touched it.
package clean

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ran-codes/notion-sync/internal/sync"
)

// presignedURLPattern matches Notion's S3 pre-signed URLs. Anchored on the
// X-Amz-Signature parameter to avoid stripping query strings from unrelated
// URLs that happen to be hosted on AWS.
//
// The pattern stops at the first whitespace, quote, parenthesis, comma, or
// closing angle bracket — these are the only characters that can terminate
// a URL inside Markdown or YAML frontmatter (which `internal/frontmatter`
// quotes file URLs with double quotes).
var presignedURLPattern = regexp.MustCompile(
	`(https?://[a-zA-Z0-9.\-]+\.amazonaws\.com/[^?\s"'<>)]*?)\?[^\s"'<>)]*X-Amz-Signature=[^\s"'<>)]*`,
)

// Result summarizes a clean run.
type Result struct {
	FilesScanned     int
	FilesChanged     int
	URLsStripped     int
	NewlinesFixed    int
	FrozenAtStripped int
	MetadataBumped   int
	DryRun           bool
}

// Folder walks `root` recursively and applies cleanups to eligible files:
//   - strips pre-signed S3 query strings from `.md` files
//   - removes `notion-frozen-at:` lines from `.md` frontmatter
//   - appends a trailing newline to `.md` and `.json` files that are missing one
//
// After the walk, any folder whose files were modified has its `_database.json`
// or `_page.json` re-stamped with the current binary's syncVersion.
//
// When dryRun is true, files are not modified; the result reports what would
// have changed.
func Folder(root string, dryRun bool) (*Result, error) {
	r := &Result{DryRun: dryRun}
	dirtyDirs := make(map[string]bool)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		name := strings.ToLower(d.Name())
		isMd := strings.HasSuffix(name, ".md")
		isJSON := strings.HasSuffix(name, ".json")

		if !isMd && !isJSON {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		changed := false
		out := string(content)

		r.FilesScanned++

		if isMd {
			stripped, count := stripContent(out)
			if count > 0 {
				out = stripped
				r.URLsStripped += count
				changed = true
			}

			defrozen, fcount := stripFrozenAt(out)
			if fcount > 0 {
				out = defrozen
				r.FrozenAtStripped += fcount
				changed = true
			}
		}

		if len(out) > 0 && out[len(out)-1] != '\n' {
			out += "\n"
			r.NewlinesFixed++
			changed = true
		}

		if !changed {
			return nil
		}

		r.FilesChanged++
		// Any cleanup in a folder — including a JSON file gaining a trailing
		// newline — marks the folder dirty so its metadata gets re-stamped.
		dirtyDirs[filepath.Dir(path)] = true

		if dryRun {
			return nil
		}

		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(out), info.Mode()); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		return nil
	})
	if err != nil {
		return r, err
	}

	// For each folder we touched, re-write its _database.json or _page.json so
	// the syncVersion field reflects the binary that performed the cleanup. This
	// gives users a record of which notion-sync version last mutated the
	// workspace. Skipped in dry-run.
	if !dryRun {
		for dir := range dirtyDirs {
			bumped, err := bumpFolderMetadata(dir)
			if err != nil {
				return r, err
			}
			if bumped {
				r.MetadataBumped++
			}
		}
	} else if sync.Version != "" {
		for dir := range dirtyDirs {
			if folderHasMetadata(dir) {
				r.MetadataBumped++
			}
		}
	}

	return r, nil
}

// folderHasMetadata reports whether a folder contains _database.json or _page.json.
func folderHasMetadata(dir string) bool {
	for _, name := range []string{sync.DatabaseMetadataFile, sync.PageMetadataFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// bumpFolderMetadata re-writes _database.json or _page.json in dir (if present)
// so the syncVersion field is stamped with the current binary's version.
// Returns true if a metadata file was rewritten.
//
// If sync.Version is empty (a misconfigured caller that didn't wire the build
// version), the bump is skipped entirely — rewriting without a version would
// produce a file with no stamp while still incrementing MetadataBumped, which
// is a misleading user-visible counter.
func bumpFolderMetadata(dir string) (bool, error) {
	if sync.Version == "" {
		return false, nil
	}
	if dbMeta, err := sync.ReadDatabaseMetadata(dir); err == nil && dbMeta != nil {
		if err := sync.WriteDatabaseMetadata(dir, dbMeta); err != nil {
			return false, fmt.Errorf("rewrite %s: %w", sync.DatabaseMetadataFile, err)
		}
		return true, nil
	}
	if pageMeta, err := sync.ReadPageMetadata(dir); err == nil && pageMeta != nil {
		if err := sync.WritePageMetadata(dir, pageMeta); err != nil {
			return false, fmt.Errorf("rewrite %s: %w", sync.PageMetadataFile, err)
		}
		return true, nil
	}
	return false, nil
}

// frozenAtKeyPattern matches a `notion-frozen-at:` line (with optional surrounding
// whitespace) inside YAML frontmatter. The key is anchored at line start so it
// won't match the substring inside another property's value.
var frozenAtKeyPattern = regexp.MustCompile(`(?m)^[ \t]*notion-frozen-at[ \t]*:.*\r?\n?`)

// stripFrozenAt removes any `notion-frozen-at:` line from the YAML frontmatter
// of a Markdown file. Body content is not touched. Returns the modified content
// and the number of lines stripped.
//
// The frontmatter is the region between the first `---` line at the start of
// the file and the next `---` line.
func stripFrozenAt(s string) (string, int) {
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return s, 0
	}
	openLen := 4
	if strings.HasPrefix(s, "---\r\n") {
		openLen = 5
	}

	rest := s[openLen:]
	closeIdx := indexOfFrontmatterClose(rest)
	if closeIdx < 0 {
		return s, 0
	}

	fmRegion := rest[:closeIdx]
	stripped := frozenAtKeyPattern.ReplaceAllString(fmRegion, "")
	count := strings.Count(fmRegion, "\n") - strings.Count(stripped, "\n")
	if count <= 0 {
		return s, 0
	}
	return s[:openLen] + stripped + rest[closeIdx:], count
}

// indexOfFrontmatterClose returns the byte offset (relative to rest) of the
// closing `---` line of YAML frontmatter, or -1 if not found.
func indexOfFrontmatterClose(rest string) int {
	for i := 0; i < len(rest); {
		// Find next newline.
		nl := strings.Index(rest[i:], "\n")
		var line string
		if nl < 0 {
			line = rest[i:]
		} else {
			line = rest[i : i+nl]
		}
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "---" {
			return i
		}
		if nl < 0 {
			return -1
		}
		i += nl + 1
	}
	return -1
}

// stripContent returns the input with all S3 pre-signed query strings removed,
// and the number of URLs that were stripped.
func stripContent(s string) (string, int) {
	count := 0
	out := presignedURLPattern.ReplaceAllStringFunc(s, func(match string) string {
		idx := strings.Index(match, "?")
		if idx < 0 {
			return match
		}
		count++
		return match[:idx]
	})
	return out, count
}
