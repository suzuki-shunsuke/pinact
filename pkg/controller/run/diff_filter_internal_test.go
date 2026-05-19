package run

import (
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseDiff(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name string
		diff string
		want map[string][]DiffLine
	}{
		{
			name: "single hunk with one + line",
			diff: `diff --git a/.github/workflows/wc-test.yaml b/.github/workflows/wc-test.yaml
index 1f703973..cc980f16 100644
--- a/.github/workflows/wc-test.yaml
+++ b/.github/workflows/wc-test.yaml
@@ -12,7 +12,7 @@ jobs:
         uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
         with:
           persist-credentials: false
-      - uses: actions/setup-go@4b73464bb391d4059bd26b0524d20df3927bd417 # v6.3.0
+      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
         with:
           go-version-file: go.mod
           cache: true
`,
			want: map[string][]DiffLine{
				".github/workflows/wc-test.yaml": {
					{Number: 15, Content: "      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0"},
				},
			},
		},
		{
			name: "new file: every body line is +",
			diff: `diff --git a/.github/workflows/new.yaml b/.github/workflows/new.yaml
new file mode 100644
index 0000000..abcdef0
--- /dev/null
+++ b/.github/workflows/new.yaml
@@ -0,0 +1,3 @@
+name: new
+on: push
+jobs: {}
`,
			want: map[string][]DiffLine{
				".github/workflows/new.yaml": {
					{Number: 1, Content: "name: new"},
					{Number: 2, Content: "on: push"},
					{Number: 3, Content: "jobs: {}"},
				},
			},
		},
		{
			name: "deleted file is excluded",
			diff: `diff --git a/.github/workflows/old.yaml b/.github/workflows/old.yaml
deleted file mode 100644
index abcdef0..0000000
--- a/.github/workflows/old.yaml
+++ /dev/null
@@ -1,3 +0,0 @@
-name: old
-on: push
-jobs: {}
`,
			want: map[string][]DiffLine{},
		},
		{
			name: "rename + edit uses new path",
			diff: `diff --git a/.github/workflows/old.yaml b/.github/workflows/new.yaml
similarity index 80%
rename from .github/workflows/old.yaml
rename to .github/workflows/new.yaml
index abcdef0..1234567 100644
--- a/.github/workflows/old.yaml
+++ b/.github/workflows/new.yaml
@@ -1,3 +1,3 @@
 name: w
-on: push
+on: pull_request
 jobs: {}
`,
			want: map[string][]DiffLine{
				".github/workflows/new.yaml": {
					{Number: 2, Content: "on: pull_request"},
				},
			},
		},
		{
			name: "multiple hunks in one file",
			diff: `diff --git a/wf.yaml b/wf.yaml
index 1111111..2222222 100644
--- a/wf.yaml
+++ b/wf.yaml
@@ -1,3 +1,3 @@
 a
-b
+B
 c
@@ -10,3 +10,3 @@ section
 x
-y
+Y
 z
`,
			want: map[string][]DiffLine{
				"wf.yaml": {
					{Number: 2, Content: "B"},
					{Number: 11, Content: "Y"},
				},
			},
		},
		{
			name: "context lines advance counter but are not targets",
			diff: `diff --git a/f b/f
index 1..2 100644
--- a/f
+++ b/f
@@ -5,5 +5,6 @@
 line5
 line6
+inserted
 line7
 line8
 line9
`,
			want: map[string][]DiffLine{
				"f": {
					{Number: 7, Content: "inserted"},
				},
			},
		},
		{
			name: "no newline at end of file marker is skipped",
			diff: `diff --git a/f b/f
index 1..2 100644
--- a/f
+++ b/f
@@ -1,2 +1,2 @@
 a
-b
\ No newline at end of file
+B
\ No newline at end of file
`,
			want: map[string][]DiffLine{
				"f": {
					{Number: 2, Content: "B"},
				},
			},
		},
		{
			name: "empty diff yields empty map",
			diff: ``,
			want: map[string][]DiffLine{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseDiff(strings.NewReader(tt.diff))
			if err != nil {
				t.Fatalf("ParseDiff() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, got.files); diff != "" {
				t.Errorf("ParseDiff() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDiffFilter_Files(t *testing.T) {
	t.Parallel()
	df := &DiffFilter{files: map[string][]DiffLine{
		"a.yaml": {{Number: 1, Content: "x"}},
		"b.yaml": {{Number: 2, Content: "y"}},
	}}
	got := df.Files()
	sort.Strings(got)
	want := []string{"a.yaml", "b.yaml"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Files() mismatch (-want +got):\n%s", diff)
	}
}

func TestDiffFilter_Has(t *testing.T) {
	t.Parallel()
	df := &DiffFilter{files: map[string][]DiffLine{"a.yaml": nil}}
	// nil-valued key still counts as present; but the parser never inserts
	// empty entries so this is just for coverage of the lookup itself.
	if !df.Has("a.yaml") {
		t.Errorf("Has(a.yaml) = false, want true")
	}
	if df.Has("c.yaml") {
		t.Errorf("Has(c.yaml) = true, want false")
	}
}

func TestDiffFilter_Lines(t *testing.T) {
	t.Parallel()
	lines := []DiffLine{{Number: 1, Content: "x"}}
	df := &DiffFilter{files: map[string][]DiffLine{"a.yaml": lines}}
	if diff := cmp.Diff(lines, df.Lines("a.yaml")); diff != "" {
		t.Errorf("Lines() mismatch (-want +got):\n%s", diff)
	}
	if got := df.Lines("c.yaml"); got != nil {
		t.Errorf("Lines(c.yaml) = %v, want nil", got)
	}
}
