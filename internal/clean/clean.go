// Package clean strips AWS S3 pre-signed query strings from already-imported
// Markdown files, without making any Notion API calls.
//
// Notion file URLs (PDFs, images, attachments) carry a query string
// (X-Amz-Signature, X-Amz-Date, X-Amz-Credential, etc.) that rotates every
// hour. The path is stable. Routine refreshes therefore produce a giant diff
// of pure URL noise. This package walks already-imported .md files and
// strips those query strings in-place, producing a one-time focused commit
// after which routine refreshes (with the default-on stripping in the sync
// path) stay clean.
package clean

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	FilesScanned int
	FilesChanged int
	URLsStripped int
	DryRun       bool
}

// Folder walks `root` recursively and strips pre-signed query strings from
// every `.md` file it finds. When dryRun is true, files are not modified;
// the result reports what would have changed.
func Folder(root string, dryRun bool) (*Result, error) {
	r := &Result{DryRun: dryRun}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		r.FilesScanned++

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		stripped, count := stripContent(string(content))
		if count == 0 {
			return nil
		}

		r.FilesChanged++
		r.URLsStripped += count

		if dryRun {
			return nil
		}

		// Preserve the existing file mode by reading then writing.
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(stripped), info.Mode()); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		return nil
	})
	if err != nil {
		return r, err
	}
	return r, nil
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
