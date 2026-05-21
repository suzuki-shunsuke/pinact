//go:build windows

package run

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestDiffFilter_Has_Windows pins the Windows-only path-separator
// conversion: filepath.Glob on Windows returns OS-native paths with `\`
// separators, and Has must still match the slash-delimited keys stored
// by ParseDiff. This complements the portable test in
// diff_filter_internal_test.go, which only exercises filepath.ToSlash
// on the host OS via filepath.FromSlash.
func TestDiffFilter_Has_Windows(t *testing.T) {
	t.Parallel()
	df := &DiffFilter{files: map[string][]DiffLine{
		".github/workflows/wc-test.yaml": nil,
	}}
	if !df.Has(`.github\workflows\wc-test.yaml`) {
		t.Errorf(`Has(.github\workflows\wc-test.yaml) = false, want true`)
	}
}

func TestDiffFilter_Lines_Windows(t *testing.T) {
	t.Parallel()
	want := []DiffLine{{Number: 2, Content: "y"}}
	df := &DiffFilter{files: map[string][]DiffLine{
		".github/workflows/wc-test.yaml": want,
	}}
	if diff := cmp.Diff(want, df.Lines(`.github\workflows\wc-test.yaml`)); diff != "" {
		t.Errorf("Lines(backslash path) mismatch (-want +got):\n%s", diff)
	}
}
