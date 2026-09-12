// Package docs holds the documents embedded in the pinact binary, which
// `pinact docs` lists and outputs so that a coding agent reads the documentation of
// the version it is actually running.
package docs

import (
	"embed"
)

// FS holds the documents. The subdirectories are embedded whole rather than by a
// pattern each, so that a document added to one of them is served by dropping the
// file in; a new subdirectory has to be added here.
//
//go:embed *.md codes upgrade_guide
var FS embed.FS
