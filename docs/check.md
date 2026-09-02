---
description: Check whether actions are pinned without editing files, online or offline. Use for `--check` and `--fix=false`, for the offline check `--no-api` and what it can and cannot detect, or to verify version comments in CI.
---

# Checking without fixing

By default, pinact edits files.

## Just validation: `--check`, `--fix=false`

If `--check` or `--fix=false` is specified, pinact just checks if actions are pinned without editing files.

```sh
pinact run --check
```

The check reports what needs pinning and exits with a non-zero [exit code](exit_codes.md), which is what makes it useful in CI.

## Offline check: `--no-api`

For an offline check (no GitHub API call, only the 40-character SHA syntactic check), add `--no-api`:

```sh
pinact run --fix=false --no-api
```

With `--no-api`, pinact can't fetch action versions and SHA, so pinact can't pin actions.
So it only checks if actions are pinned with full-length commit SHA.

`--no-api` requires `--fix=false` (or `--format sarif`), since there is nothing pinact could fix without the API.

Note that `--no-api` also means pinact can't resolve a SHA to its tag, so an action pinned to a bare SHA without a version comment is reported as an error.
[See SHA-pinned action requires a version comment](codes/005.md).

## Verify version comments: `--verify-comment` (`--verify`, `-v`)

```sh
pinact run --verify-comment
```

This checks that a version comment names the version its SHA actually is, which a comment written by hand or by an attacker doesn't have to.
[Please see `Verify version comments`.](codes/001.md)
