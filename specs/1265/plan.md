# Implementation Plan: Add `--cooldown` option to `pinact run`

## Files to Modify

### 1. `pkg/cli/run/command.go`
- Add `Cooldown int` field to `Flags` struct
- Add `--cooldown` IntFlag definition with `Validator`
- Add validation in `action()` method for flag combination

### 2. `pkg/controller/run/run.go`
- Add `Cooldown int` field to `ParamRun` struct

### 3. `pkg/controller/run/github.go`
- Add `GitService` interface for getting commits
- Modify `getLatestVersionFromReleases()` to filter releases by `PublishedAt` date
- Modify `getLatestVersionFromTags()` to filter tags by commit date
- Add debug logging when a version is skipped due to cooldown

## Implementation Details

### CLI Flag Definition
```go
&cli.IntFlag{
    Name:        "cooldown",
    Usage:       "Skip versions released within the specified number of days (requires -u)",
    Destination: &flags.Cooldown,
    Validator: func(i int) error {
        if i < 0 {
            return errors.New("--cooldown must be a non-negative integer")
        }
        return nil
    },
},
```

### Validation Logic (in `action()`)
```go
if flags.Cooldown > 0 && !flags.Update {
    return errors.New("--cooldown requires --update (-u) flag")
}
```

### Cooldown Cutoff Calculation
Calculate `cutoff` once in `getLatestVersion()` and pass to filtering functions:
```go
func (c *Controller) getLatestVersion(ctx context.Context, logger *slog.Logger, owner, repo, currentVersion string) (string, error) {
    isStable := isStableVersion(currentVersion)

    // Calculate cutoff once
    var cutoff time.Time
    if c.param.Cooldown > 0 {
        cutoff = time.Now().AddDate(0, 0, -c.param.Cooldown)
    }

    lv, err := c.getLatestVersionFromReleases(ctx, logger, owner, repo, isStable, cutoff)
    // ...
    return c.getLatestVersionFromTags(ctx, logger, owner, repo, isStable, cutoff)
}
```

### Release Filtering Logic
```go
// cutoff is passed as parameter (zero value means no filtering)
if !cutoff.IsZero() && release.GetPublishedAt().Time.After(cutoff) {
    logger.Debug("skip release due to cooldown",
        slog.String("tag", tag),
        slog.Time("published_at", release.GetPublishedAt().Time))
    continue
}
```

### Tag Handling
- Use `GitService.GetCommit(ctx, owner, repo, sha)` to get commit info
- Check `commit.GetCommitter().GetDate().Time` against `cutoff` parameter
- Cache results to avoid redundant API calls

## Execution Order

1. Add `Cooldown int` field to `Flags` struct in `pkg/cli/run/command.go`
2. Add `--cooldown` IntFlag definition with `Validator`
3. Add validation in `action()` method for flag combination
4. Add `Cooldown int` field to `ParamRun` struct in `pkg/controller/run/run.go`
5. Pass `Cooldown` to `ParamRun` in `action()` method
6. Add `GitService` interface to `pkg/controller/run/github.go`
7. Implement cooldown filtering in `getLatestVersionFromReleases()`
8. Implement cooldown filtering in `getLatestVersionFromTags()`
9. Wire up `gh.Git` to controller in `action()` method
10. Run `cmdx v` and `cmdx t` to validate
