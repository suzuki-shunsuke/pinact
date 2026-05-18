package run

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

// DiffLine represents a single added line (post-image) extracted from a
// unified diff. Number is the 1-based line number in the new file.
type DiffLine struct {
	Number  int
	Content string
}

// DiffFilter holds the set of added lines per file path, parsed from a
// unified diff. When set on ParamRun, pinact processes only these lines.
type DiffFilter struct {
	files map[string][]DiffLine
}

// ParseDiff reads a unified diff from r and returns the set of `+` lines
// per file (keyed by post-image path with `a/` / `b/` prefixes stripped).
//
// Files renamed to /dev/null (deletions) are excluded. Renames are tracked
// under the new path. A file with no `+` lines (rename-only, mode change)
// is excluded.
func ParseDiff(r io.Reader) (*DiffFilter, error) {
	fileDiffs, err := diff.NewMultiFileDiffReader(r).ReadAllFiles()
	if err != nil {
		return nil, fmt.Errorf("parse unified diff: %w", err)
	}
	files := make(map[string][]DiffLine)
	for _, fd := range fileDiffs {
		path := newPath(fd)
		if path == "" {
			continue
		}
		lines := addedLines(fd)
		if len(lines) == 0 {
			continue
		}
		files[path] = append(files[path], lines...)
	}
	return &DiffFilter{files: files}, nil
}

// Files returns the set of file paths covered by this filter.
func (f *DiffFilter) Files() []string {
	paths := make([]string, 0, len(f.files))
	for p := range f.files {
		paths = append(paths, p)
	}
	return paths
}

// Lines returns the `+` lines recorded for the given path, or nil if the
// path is not in the filter.
func (f *DiffFilter) Lines(path string) []DiffLine {
	return f.files[path]
}

// Has reports whether the filter contains any `+` lines for path.
func (f *DiffFilter) Has(path string) bool {
	_, ok := f.files[path]
	return ok
}

// newPath returns the post-image path for a FileDiff with `a/` / `b/`
// prefixes stripped. Returns "" for deleted files (NewName == /dev/null).
func newPath(fd *diff.FileDiff) string {
	if fd.NewName == "" || fd.NewName == "/dev/null" {
		return ""
	}
	return stripDiffPathPrefix(fd.NewName)
}

// stripDiffPathPrefix removes a leading `a/` or `b/` from a unified diff
// path. Other prefixes are returned as-is.
func stripDiffPathPrefix(p string) string {
	switch {
	case strings.HasPrefix(p, "a/"):
		return p[2:]
	case strings.HasPrefix(p, "b/"):
		return p[2:]
	}
	return p
}

// addedLines walks every hunk in fd and returns the post-image line numbers
// and contents of `+` lines. New-file line numbers start at hunk.NewStartLine
// and advance for `+` and ` ` (context) prefixes; `-` lines do not advance
// the counter, and `\` (no-newline marker) lines are skipped.
func addedLines(fd *diff.FileDiff) []DiffLine {
	var out []DiffLine
	for _, h := range fd.Hunks {
		newLine := int(h.NewStartLine)
		for raw := range bytes.SplitSeq(h.Body, []byte{'\n'}) {
			if len(raw) == 0 {
				continue
			}
			switch raw[0] {
			case '+':
				out = append(out, DiffLine{
					Number:  newLine,
					Content: string(raw[1:]),
				})
				newLine++
			case ' ':
				newLine++
			case '-':
				// removed line: new-file counter does not advance
			case '\\':
				// "\ No newline at end of file" marker: skip
			}
		}
	}
	return out
}
