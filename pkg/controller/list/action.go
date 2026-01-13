package list

// ActionInfo contains information about a GitHub Action or reusable workflow
// extracted from a workflow file. It is used for template rendering.
type ActionInfo struct {
	ActionName string // Full action name (owner/repo or owner/repo/path)
	RepoOwner  string // Repository owner
	RepoName   string // Repository name
	Version    string // Version/ref (tag, branch, or commit SHA)
	Comment    string // Version comment (e.g., v4.0.0)
	FilePath   string // Full path to the workflow file
	FileName   string // Base name of the workflow file
	LineNumber int    // Line number in the file
}
