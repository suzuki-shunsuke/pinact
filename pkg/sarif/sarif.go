package sarif

// Log represents the top-level SARIF log object.
// https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html
type Log struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

// Run represents a single run of an analysis tool.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool describes the analysis tool that produced the results.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver describes the tool component that produced the results.
type Driver struct {
	Name           string `json:"name"`
	InformationURI string `json:"informationUri,omitempty"`
	Version        string `json:"version,omitempty"`
	Rules          []Rule `json:"rules,omitempty"`
}

// Rule describes an analysis rule.
type Rule struct {
	ID               string  `json:"id"`
	ShortDescription Message `json:"shortDescription"`
}

// Result represents a single result from the analysis.
type Result struct {
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations"`
}

// Message contains text describing a result or rule.
type Message struct {
	Text string `json:"text"`
}

// Location describes a location relevant to a result.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation describes a physical location in a file.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region"`
}

// ArtifactLocation describes the location of an artifact.
type ArtifactLocation struct {
	URI string `json:"uri"`
}

// Region describes a region within an artifact.
type Region struct {
	StartLine int `json:"startLine"`
}
