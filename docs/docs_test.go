package docs_test

import (
	"strings"
	"testing"

	cobradocs "github.com/suzuki-shunsuke/cobra-util/docs"
	docsfs "github.com/suzuki-shunsuke/pinact/v5/docs"
)

// TestFS checks that every embedded document has a frontmatter with a description.
// `pinact docs list` fails on a document that hasn't, and a document is added by
// dropping a Markdown file in this directory, so nothing else would catch it before
// a coding agent runs the command.
func TestFS(t *testing.T) {
	t.Parallel()
	names, err := cobradocs.Names(docsfs.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no document is embedded")
	}
	for _, name := range names {
		b, err := docsfs.FS.ReadFile(name + cobradocs.Ext)
		if err != nil {
			t.Fatal(err)
		}
		result := &cobradocs.Result{Name: name}
		if err := cobradocs.Parse(b, result); err != nil {
			t.Fatalf("parse the frontmatter of %s: %v", name, err)
		}
		if strings.TrimSpace(result.Description) == "" {
			t.Fatalf("the document %s has no description", name)
		}
	}
}
