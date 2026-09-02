---
description: Run pinact to pin the actions and reusable workflows of a repository. Use for which files are pinned when none is given, for pinning actions written in Markdown or any other text file, or as the entry point to the run options.
---

# Usage

```sh
pinact run [<workflow file>...]
```

If no file is specified, the following files are pinned:

```
.github/workflows/*.yml
.github/workflows/*.yaml
action.yml
action.yaml
*/action.yml
*/action.yaml
*/*/action.yml
*/*/action.yaml
*/*/*/action.yml
*/*/*/action.yaml
```

The list of target files is configurable with [`files` in the configuration file](config.md).

pinact calls the GitHub API to fetch releases and tags.
To avoid API rate limiting, you should [pass a GitHub access token](github-token.md).

## Fix example codes in documents

Not only workflow files, but also text files of any formats are supported.
This is useful to pin actions in text files such as `README.md`.

```sh
pinact run README.md
```

## Options

- [Check without editing files](check.md): `--check`, `--fix=false`, `--no-api`
- [Update actions](update.md): `--update`, `--min-age`, `--verify-min-age`
- [Verify version comments](codes/001.md): `--verify-comment`
- [Pin branches](branch-to-tag.md): `--branch-to-tag`
- [Include and exclude specific actions](include-exclude.md): `--include`, `--exclude`
- [Generate SARIF](sarif.md): `--format sarif`
- [Pin only changed lines](diff-file.md): `--diff-file`

Every option is also listed in [USAGE.md](../USAGE.md), which is the help of every command.

## Exit codes

[See Exit codes](exit-codes.md).
