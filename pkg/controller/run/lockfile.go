package run

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"

	parserlock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"github.com/spf13/afero"
)

// lockfileState holds the GitHub Actions workflow dependency lockfile
// .github/workflows/actions.lock.
// https://docs.github.com/en/enterprise-cloud@latest/actions/concepts/security/workflow-dependency-lockfiles
//
// A nil *lockfileState means the lockfile doesn't exist, so pinact pins actions as usual.
type lockfileState struct {
	file parserlock.File
	// err is the error pinact got when reading the lockfile.
	err error
}

// readLockfile reads the lockfile for the given workflow keys.
// Required fields are validated only for entries those workflows reference,
// so a broken entry that no workflow uses doesn't make the whole lockfile unreadable.
// It returns nil if the lockfile doesn't exist or no workflow key is given.
// If the lockfile exists but can't be read, the returned state has the error.
func readLockfile(afs afero.Fs, workflowKeys []string) *lockfileState {
	if len(workflowKeys) == 0 {
		// The parser validates all entries if no workflow file path is given.
		return nil
	}
	b, err := afero.ReadFile(afs, parserlock.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return &lockfileState{err: fmt.Errorf("read the lockfile: %w", err)}
	}
	f, err := parserlock.Parse(b, workflowKeys...)
	if err != nil {
		return &lockfileState{err: fmt.Errorf("parse the lockfile: %w", err)}
	}
	return &lockfileState{file: f}
}

// workflowKey returns the path of a workflow file relative to the repository root,
// which is how the lockfile refers to workflows.
// It returns an empty string if the file isn't a workflow file .github/workflows/*.y(a)ml.
// cwd is the repository root.
func workflowKey(cwd, filePath string) string {
	if filepath.IsAbs(filePath) {
		rel, err := filepath.Rel(cwd, filePath)
		if err != nil {
			return ""
		}
		filePath = rel
	}
	key := path.Clean(filepath.ToSlash(filePath))
	if path.Dir(key) != ".github/workflows" {
		return ""
	}
	if ext := path.Ext(key); ext != ".yml" && ext != ".yaml" {
		return ""
	}
	return key
}

// checkLockfile decides how pinact handles an action in a workflow file using the lockfile.
// It returns false if the lockfile doesn't concern the action, so pinact processes it as usual:
// the file isn't a workflow file, the action is a reusable workflow, the lockfile doesn't exist,
// or the workflow isn't onboarded to the lockfile.
// Otherwise it returns true and pinact doesn't change the action:
//   - The action is locked: no error.
//   - The workflow is onboarded but the action isn't locked: GitHub Actions rejects the workflow,
//     so it returns an error asking to regenerate the lockfile.
//   - The lockfile can't be read: pinact can't tell whether the action is locked.
//     An action pinned by full commit SHA is left as is, and otherwise it returns an error.
func (s *lockfileState) checkLockfile(key string, action *Action) (bool, error) {
	if s == nil || key == "" {
		return false, nil
	}
	uses := action.Name + "@" + action.Version
	ref := parserlock.ParseActionRef(uses)
	if ref == nil {
		// Reusable workflows aren't locked by the lockfile.
		return false, nil
	}
	if s.err != nil {
		if getVersionType(action.Version) == FullCommitSHA {
			return true, nil
		}
		return true, fmt.Errorf("%w: the lockfile couldn't be read, so pinact can't tell whether %s is locked. Fix the lockfile, or update pinact if the lockfile's schema version is newer", ErrUnfixable, uses)
	}
	keys, ok := s.file.LookupWorkflow(key)
	if !ok {
		return false, nil
	}
	pin := parserlock.IndexKey(ref.Owner, ref.Repo, ref.Ref)
	if slices.Contains(keys, pin) {
		if _, ok := s.file.Dependencies[pin]; ok {
			return true, nil
		}
	}
	return true, fmt.Errorf("%w: the lockfile is missing %s. Regenerate it by running gh actions-lock", ErrUnfixable, pin)
}
