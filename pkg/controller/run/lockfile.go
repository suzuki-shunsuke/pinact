package run

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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

// lockfileTarget is the lockfile that concerns a target file.
type lockfileTarget struct {
	state *lockfileState
	// keys are the workflow keys whose lock state concerns the file:
	// the workflow itself for a workflow file, and the workflows using an action file
	// as a self repository action `$/…` for the action file.
	keys []string
}

// readLockfile reads the lockfile lockfilePath for the given workflow keys.
// Required fields are validated only for entries those workflows reference,
// so a broken entry that no workflow uses doesn't make the whole lockfile unreadable.
// It returns nil if the lockfile doesn't exist or no workflow key is given.
// If the lockfile exists but can't be read, the returned state has the error.
func readLockfile(afs afero.Fs, lockfilePath string, workflowKeys []string) *lockfileState {
	if len(workflowKeys) == 0 {
		// The parser validates all entries if no workflow file path is given.
		return nil
	}
	b, err := afero.ReadFile(afs, lockfilePath)
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

// workflowDirAndKey returns the directory of a workflow file and its path relative to the repository root,
// which is how the lockfile refers to workflows.
// The repository root is derived from the file path, so it doesn't depend on the working directory.
// For instance, /repo/.github/workflows/ci.yml returns /repo/.github/workflows and .github/workflows/ci.yml.
// It returns empty strings if the file isn't a workflow file .github/workflows/*.y(a)ml.
func workflowDirAndKey(filePath string) (string, string) {
	ext := filepath.Ext(filePath)
	if ext != ".yml" && ext != ".yaml" {
		return "", ""
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", ""
	}
	dir := filepath.Dir(abs)
	if filepath.Base(dir) != "workflows" || filepath.Base(filepath.Dir(dir)) != ".github" {
		return "", ""
	}
	return dir, ".github/workflows/" + filepath.Base(abs)
}

// lockfileRoot collects the target files of a repository having a lockfile.
type lockfileRoot struct {
	// targets maps a target file path to the workflow keys concerning it.
	targets map[string][]string
	// actionFiles maps the path of a target action file to its absolute path.
	actionFiles map[string]string
}

// readLockfiles reads the lockfile .github/workflows/actions.lock of the repository of each target file,
// and returns the lockfile and the workflow keys concerning each target file.
//
// For a workflow file, the repository root is derived from its path.
// For an action file, the nearest ancestor directory having the lockfile is the repository root,
// and the action file concerns the workflows using it as a self repository action `$/…`,
// because the lockfile locks the actions the action file uses as dependencies of those workflows.
// Each lockfile is read once with the keys of the workflows concerning the target files.
func readLockfiles(afs afero.Fs, filePaths []string) map[string]*lockfileTarget {
	targets := map[string]*lockfileTarget{}
	for root, r := range lockfileRoots(afs, filePaths) {
		r.readTargets(afs, root, targets)
	}
	return targets
}

// lockfileRoots groups the target files by the root of the repository having the lockfile.
func lockfileRoots(afs afero.Fs, filePaths []string) map[string]*lockfileRoot {
	roots := map[string]*lockfileRoot{}
	getRoot := func(root string) *lockfileRoot {
		r, ok := roots[root]
		if !ok {
			r = &lockfileRoot{targets: map[string][]string{}, actionFiles: map[string]string{}}
			roots[root] = r
		}
		return r
	}
	for _, p := range filePaths {
		if dir, key := workflowDirAndKey(p); key != "" {
			getRoot(filepath.Dir(filepath.Dir(dir))).targets[p] = []string{key}
			continue
		}
		if !isActionFile(p) {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if root := findLockfileRoot(afs, filepath.Dir(abs)); root != "" {
			getRoot(root).actionFiles[p] = abs
		}
	}
	return roots
}

// readTargets reads the lockfile of the repository root and adds the target files concerning it to targets.
func (r *lockfileRoot) readTargets(afs afero.Fs, root string, targets map[string]*lockfileTarget) {
	if len(r.actionFiles) > 0 {
		selfRefs := scanSelfRepositoryActions(afs, root)
		for p, abs := range r.actionFiles {
			if keys := selfRefs[abs]; len(keys) > 0 {
				r.targets[p] = keys
			}
		}
	}
	var keys []string
	for _, ks := range r.targets {
		for _, k := range ks {
			if !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
	}
	state := readLockfile(afs, filepath.Join(root, filepath.FromSlash(parserlock.Path)), keys)
	if state == nil {
		return
	}
	for p, ks := range r.targets {
		targets[p] = &lockfileTarget{state: state, keys: ks}
	}
}

// actionFileNames are the file names of action files.
var actionFileNames = []string{"action.yml", "action.yaml"} //nolint:gochecknoglobals // read-only list

// isActionFile reports whether the file is an action file action.y(a)ml.
func isActionFile(filePath string) bool {
	return slices.Contains(actionFileNames, filepath.Base(filePath))
}

// findLockfileRoot returns the nearest directory from dir to the file system root having the lockfile,
// or an empty string if no directory has it.
func findLockfileRoot(afs afero.Fs, dir string) string {
	for {
		if _, err := afs.Stat(filepath.Join(dir, filepath.FromSlash(parserlock.Path))); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// selfRepositoryUsesPattern matches a `uses` line of a self repository reference `$/…`.
var selfRepositoryUsesPattern = regexp.MustCompile(`^ *(?:- +)?['"]?uses['"]? *: +['"]?\$/([^'" #]+)`)

// scanSelfRepositoryActions finds the action files that workflows in the repository root use
// as self repository actions `$/…`, following `$/…` in the action files recursively,
// and returns the keys of the workflows using each action file.
// The keys are keyed by the absolute path of the action file.
// Reusable workflows called with `$/…` aren't followed because they are workflows themselves.
func scanSelfRepositoryActions(afs afero.Fs, root string) map[string][]string {
	refs := map[string][]string{}
	for _, pattern := range []string{"*.yml", "*.yaml"} {
		files, err := afero.Glob(afs, filepath.Join(root, ".github", "workflows", pattern))
		if err != nil {
			continue
		}
		for _, f := range files {
			key := ".github/workflows/" + filepath.Base(f)
			for _, actionFile := range followSelfRepositoryActions(afs, root, f) {
				refs[actionFile] = append(refs[actionFile], key)
			}
		}
	}
	return refs
}

// followSelfRepositoryActions returns the action files that the file uses as self repository actions `$/…`,
// following `$/…` in the action files recursively.
func followSelfRepositoryActions(afs afero.Fs, root, filePath string) []string {
	var actionFiles []string
	seen := map[string]struct{}{}
	pending := []string{filePath}
	for len(pending) > 0 {
		f := pending[0]
		pending = pending[1:]
		for _, rel := range selfRepositoryRefs(afs, f) {
			actionFile := selfRepositoryActionFile(afs, root, rel)
			if actionFile == "" {
				continue
			}
			if _, ok := seen[actionFile]; ok {
				continue
			}
			seen[actionFile] = struct{}{}
			actionFiles = append(actionFiles, actionFile)
			pending = append(pending, actionFile)
		}
	}
	return actionFiles
}

// selfRepositoryRefs returns the paths of self repository references `$/<path>` in the file.
// References with `@` are invalid and ignored.
func selfRepositoryRefs(afs afero.Fs, filePath string) []string {
	b, err := afero.ReadFile(afs, filePath)
	if err != nil {
		return nil
	}
	var refs []string
	for line := range strings.Lines(string(b)) {
		m := selfRepositoryUsesPattern.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		if m == nil || strings.Contains(m[1], "@") {
			continue
		}
		refs = append(refs, m[1])
	}
	return refs
}

// selfRepositoryActionFile returns the absolute path of the action file of a self repository reference `$/<rel>`,
// or an empty string if it isn't an action in the repository, for instance a reusable workflow.
func selfRepositoryActionFile(afs afero.Fs, root, rel string) string {
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if r, err := filepath.Rel(root, dir); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return ""
	}
	for _, name := range actionFileNames {
		f := filepath.Join(dir, name)
		if _, err := afs.Stat(f); err == nil {
			return f
		}
	}
	return ""
}

// checkLockfile checks the action in the target file with the lockfile concerning the file.
// See lockfileState.checkLockfile.
func (c *Controller) checkLockfile(filePath string, action *Action) (bool, error) {
	t := c.lockfiles[filePath]
	if t == nil {
		return false, nil
	}
	return t.state.checkLockfile(t.keys, action)
}

// checkLockfile decides how pinact handles an action in a target file using the lockfile.
// keys are the workflow keys concerning the file.
// It returns false if the lockfile doesn't concern the action, so pinact processes it as usual:
// the action is a reusable workflow, the lockfile doesn't exist, or no workflow of keys is onboarded to the lockfile.
// Otherwise it returns true and pinact doesn't change the action:
//   - The action is locked for all onboarded workflows of keys: no error.
//   - The action isn't locked for an onboarded workflow: GitHub Actions rejects the workflow,
//     so it returns an error asking to regenerate the lockfile.
//   - The lockfile can't be read: pinact can't tell whether the action is locked.
//     An action pinned by full commit SHA is left as is, and otherwise it returns an error.
func (s *lockfileState) checkLockfile(keys []string, action *Action) (bool, error) {
	if s == nil || len(keys) == 0 {
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
	pin := parserlock.IndexKey(ref.Owner, ref.Repo, ref.Ref)
	handled := false
	for _, key := range keys {
		pins, ok := s.file.LookupWorkflow(key)
		if !ok {
			continue
		}
		handled = true
		if !s.locked(pins, pin) {
			return true, fmt.Errorf("%w: the lockfile is missing %s for %s. Regenerate it by running gh actions-lock", ErrUnfixable, pin, key)
		}
	}
	return handled, nil
}

// locked reports whether pin is in the pins of a workflow and in the dependencies.
func (s *lockfileState) locked(pins []string, pin string) bool {
	if !slices.Contains(pins, pin) {
		return false
	}
	_, ok := s.file.Dependencies[pin]
	return ok
}
