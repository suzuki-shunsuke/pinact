package run

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	parserlock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// lockfileState holds the GitHub Actions workflow dependency lockfile
// .github/workflows/actions.lock.
// https://docs.github.com/en/enterprise-cloud@latest/actions/concepts/security/workflow-dependency-lockfiles
//
// See https://github.com/suzuki-shunsuke/pinact/issues/1828 for the spec.
type lockfileState struct {
	file parserlock.File
}

// lockfileTarget is the lockfile that concerns a target file.
// pinact doesn't change actions in workflow steps of the file.
type lockfileTarget struct {
	state *lockfileState
	// keys are the keys of the onboarded workflows concerning the file:
	// the workflow itself for a workflow file, and the onboarded workflows using an action file
	// as a self repository action `$/…` for the action file.
	keys []string
}

// readLockfile reads the lockfile lockfilePath for the given workflow keys.
// Required fields are validated only for entries those workflows reference,
// so a broken entry that no workflow uses doesn't make the whole lockfile unreadable.
// It returns nil without an error if the lockfile doesn't exist or no workflow key is given.
func readLockfile(afs afero.Fs, lockfilePath string, workflowKeys []string) (*lockfileState, error) {
	if len(workflowKeys) == 0 {
		// The parser validates all entries if no workflow file path is given.
		return nil, nil //nolint:nilnil
	}
	b, err := afero.ReadFile(afs, lockfilePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil //nolint:nilnil
		}
		return nil, fmt.Errorf("read the lockfile: %w", err)
	}
	f, err := parserlock.Parse(b, workflowKeys...)
	if err != nil {
		return nil, fmt.Errorf("parse the lockfile: %w", err)
	}
	return &lockfileState{file: f}, nil
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
// and returns the target files whose actions in workflow steps pinact doesn't change, with their lockfiles.
//
// For a workflow file, the repository root is derived from its path, and the file is a target if it's onboarded.
// For an action file, the nearest ancestor directory having the lockfile is the repository root,
// and the file is a target if an onboarded workflow uses it as a self repository action `$/…`,
// because the lockfile locks the actions the action file uses as dependencies of the workflow.
// It outputs a warning for each workflow concerning a target file that isn't onboarded,
// because usually all workflows should be onboarded and `gh actions-lock` may not have been run.
// Each lockfile is read once with the keys of the workflows concerning the target files.
// It returns an error if a lockfile can't be read, because pinact can't tell which workflows are onboarded.
func readLockfiles(afs afero.Fs, logger *slog.Logger, filePaths []string) (map[string]*lockfileTarget, error) {
	targets := map[string]*lockfileTarget{}
	for root, r := range lockfileRoots(afs, filePaths) {
		if err := r.readTargets(afs, logger, root, targets); err != nil {
			return nil, slogerr.With(err, "lockfile", filepath.Join(root, filepath.FromSlash(parserlock.Path))) //nolint:wrapcheck
		}
	}
	return targets, nil
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
func (r *lockfileRoot) readTargets(afs afero.Fs, logger *slog.Logger, root string, targets map[string]*lockfileTarget) error {
	r.addSelfRepositoryActions(afs, root)
	lockfilePath := filepath.Join(root, filepath.FromSlash(parserlock.Path))
	state, err := readLockfile(afs, lockfilePath, r.keys())
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	warned := map[string]struct{}{}
	for p, ks := range r.targets {
		onboarded := state.onboarded(ks)
		warnNotOnboarded(logger, lockfilePath, ks, onboarded, warned)
		if len(onboarded) > 0 {
			targets[p] = &lockfileTarget{state: state, keys: onboarded}
		}
	}
	return nil
}

// addSelfRepositoryActions adds the target action files used as self repository actions `$/…`
// by workflows in the repository root to the targets, with the keys of those workflows.
func (r *lockfileRoot) addSelfRepositoryActions(afs afero.Fs, root string) {
	if len(r.actionFiles) == 0 {
		return
	}
	selfRefs := scanSelfRepositoryActions(afs, root)
	for p, abs := range r.actionFiles {
		if keys := selfRefs[abs]; len(keys) > 0 {
			r.targets[p] = keys
		}
	}
}

// warnNotOnboarded outputs a warning for each workflow of keys that isn't onboarded, once per workflow.
// Usually all workflows should be onboarded, so it's likely that `gh actions-lock` wasn't run.
func warnNotOnboarded(logger *slog.Logger, lockfilePath string, keys, onboarded []string, warned map[string]struct{}) {
	for _, k := range keys {
		if slices.Contains(onboarded, k) {
			continue
		}
		if _, ok := warned[k]; ok {
			continue
		}
		warned[k] = struct{}{}
		logger.Warn("the workflow isn't in the lockfile. Run gh actions-lock to add it", "workflow", k, "lockfile", lockfilePath)
	}
}

// keys returns the keys of the workflows concerning the target files without duplicates.
func (r *lockfileRoot) keys() []string {
	var keys []string
	for _, ks := range r.targets {
		for _, k := range ks {
			if !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
	}
	return keys
}

// onboarded returns the keys of the workflows onboarded to the lockfile among keys.
func (s *lockfileState) onboarded(keys []string) []string {
	var onboarded []string
	for _, k := range keys {
		if _, ok := s.file.LookupWorkflow(k); ok {
			onboarded = append(onboarded, k)
		}
	}
	return onboarded
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

// leaveToLockfile reports whether pinact leaves the action in the target file to the lockfile,
// and outputs a warning if the lockfile doesn't record the action.
func (c *Controller) leaveToLockfile(logger *slog.Logger, filePath string, action *Action) bool {
	handled, warning := c.checkLockfile(filePath, action)
	if !handled {
		return false
	}
	if warning != "" {
		logger.Warn(warning)
	}
	logger.Debug("leave the action to the lockfile")
	return true
}

// checkLockfile reports whether pinact leaves the action in the target file to the lockfile.
// If it does, it also returns a warning message if the lockfile doesn't record the action.
// See lockfileState.checkLockfile.
func (c *Controller) checkLockfile(filePath string, action *Action) (bool, string) {
	t := c.lockfiles[filePath]
	if t == nil {
		return false, ""
	}
	return t.state.checkLockfile(t.keys, action)
}

// checkLockfile reports whether pinact leaves the action to the lockfile.
// keys are the onboarded workflows concerning the file having the action.
// It returns false for reusable workflows, which the lockfile doesn't lock, so pinact pins them as usual.
// Otherwise it returns true, and a warning message if the lockfile doesn't record the action for some of the workflows.
// It's a warning, not an error, because GitHub Actions rejects such workflows and
// it's likely that `gh actions-lock` wasn't run.
func (s *lockfileState) checkLockfile(keys []string, action *Action) (bool, string) {
	if s == nil || len(keys) == 0 {
		return false, ""
	}
	ref := parserlock.ParseActionRef(action.Name + "@" + action.Version)
	if ref == nil {
		// Reusable workflows aren't locked by the lockfile.
		return false, ""
	}
	pin := parserlock.IndexKey(ref.Owner, ref.Repo, ref.Ref)
	var missing []string
	for _, key := range keys {
		pins, _ := s.file.LookupWorkflow(key)
		if !s.locked(pins, pin) {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return true, ""
	}
	return true, fmt.Sprintf("the lockfile doesn't record %s for %s, so GitHub Actions rejects the workflow. Run gh actions-lock to regenerate the lockfile", pin, strings.Join(missing, ", "))
}

// locked reports whether pin is in the pins of a workflow and in the dependencies.
func (s *lockfileState) locked(pins []string, pin string) bool {
	if !slices.Contains(pins, pin) {
		return false
	}
	_, ok := s.file.Dependencies[pin]
	return ok
}
