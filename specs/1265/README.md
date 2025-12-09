# Spec: Add `--min-age` option to `pinact run`

- [#1265](https://github.com/suzuki-shunsuke/pinact/pull/1265)
- [#1266](https://github.com/suzuki-shunsuke/pinact/pull/1266)
- [#1267](https://github.com/suzuki-shunsuke/pinact/pull/1267)

## Overview

Add a `--min-age` (`-m`) option to `pinact run` that works in conjunction with the `-u` (update) option. This option filters update targets based on release age - only versions released more than the specified number of days ago will be considered for updates.

## Behavior

- When `--min-age N` is specified, versions released within the last N days are skipped
- Example: `pinact run -u --min-age 7` will exclude any versions released within the past 7 days from updates
- The `-u` option fetches GitHub Releases or tags via GitHub API; this feature checks the release/tag creation date against the min-age period
- When a version is skipped due to min-age, output a debug log message

## Validation Rules

- **Error**: If `--min-age` is specified without `-u` option
- **Error**: If `--min-age` is given a negative value
- **Default**: If `--min-age` is not specified or `--min-age 0`, all versions are eligible for update (existing behavior)

## Use Case

This feature helps avoid updating to recently released, potentially unstable versions, allowing users to update only to versions that have had time to prove their stability.

## Example Usage

```sh
pinact run -u --min-age 7
# or using the short alias
pinact run -u -m 7
```

## Date Fields Used

- **Releases**: Use `PublishedAt` field from GitHub API
- **Tags**: Use `Committer.Date` from the commit object (requires additional API call to `GET /repos/{owner}/{repo}/git/commits/{sha}`)
  - If the commit cannot be fetched, the tag is skipped and a warning is logged
