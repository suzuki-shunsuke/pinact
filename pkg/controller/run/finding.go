package run

// Finding represents a single finding from the analysis.
type Finding struct {
	File    string
	Line    int
	OldLine string
	NewLine string
	Message string
}
