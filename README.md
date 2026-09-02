# pinact

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/suzuki-shunsuke/pinact)
[Install](INSTALL.md) | [Usage](docs/usage.md) | [Configuration](docs/config.md) | [Agent Skill](#installing-the-agent-skill)

pinact is a CLI to pin GitHub Actions and Reusable Workflows.
pinact can also [update their versions](docs/update.md) and [verify version comments](docs/codes/001.md).

> [!NOTE]
> If you are a coding agent and pinact is installed, run `pinact docs list` to list the documentation and `pinact docs show <name>` to read it before answering questions about pinact or troubleshooting its errors.
> The embedded documentation matches the installed version. If pinact isn't installed, read the documents listed in [Documentation](#documentation).

```diff
$ pinact run
.github/workflows/test.yaml:8
-       - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3
+       - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1
.github/workflows/test.yaml:9
-       - uses: actions/setup-go@v4
+       - uses: actions/setup-go@7b8cf10d4e4a01d4992d18a89f4d7dc5a3e6d6f4 # v4.3.0
.github/workflows/test.yaml:10
-       - uses: actions/cache@v3.3.1
+       - uses: actions/cache@88522ab9f39a2ea568f7027eddc7d8d8bc9d59c8 # v3.3.1
.github/workflows/test.yaml:16
-     uses: suzuki-shunsuke/actionlint-workflow/.github/workflows/actionlint.yaml@v0.5.0
+     uses: suzuki-shunsuke/actionlint-workflow/.github/workflows/actionlint.yaml@b6a5f966d4504893b2aeb60cf2b0de8946e48504 # v0.5.0
```

## Features

1. Pin GitHub Actions and Reusable Workflows
1. [Check if actions are pinned without editing files](docs/check.md)
1. [Offline check without GitHub API](docs/check.md)
1. [Update actions](docs/update.md) with a [minimum release age](docs/update.md)
1. [Verify version comments](docs/codes/001.md)
1. [Require a version comment on SHA-pinned actions](docs/codes/005.md)
1. [Verify if actions meet the minimum release age](docs/update.md)
1. [Pin branches](docs/branch_to_tag.md)
1. [Include and exclude specific actions](docs/include_exclude.md)
1. [Generate SARIF](docs/sarif.md). This is useful to create reviews using [reviewdog](docs/sarif.md)
1. [Read GitHub access token via keyrings or ghtkn](docs/github_token.md)
1. [Pin only changed lines](docs/diff_file.md)
1. [Support GitHub Enterprise Server](docs/ghes.md)
1. [GitHub Action](https://github.com/suzuki-shunsuke/pinact-action)

## Getting Started

1. [Install pinact](INSTALL.md)

2. Pin the actions of a repository:

```sh
pinact run
```

Without an argument, pinact pins the workflow files and the action files of the repository; [Usage](docs/usage.md) lists them and shows how to pin actions written in a document such as `README.md`.

3. Check them in CI instead of fixing them:

```sh
pinact run --check
```

The run [exits with a non-zero code](docs/exit_codes.md) when something needs pinning. [Checking without fixing](docs/check.md) covers `--check`, `--fix=false`, and the offline check `--no-api`.

4. Pass a GitHub access token so the API calls aren't rate limited:

```sh
export GITHUB_TOKEN=<your token>
```

pinact can also read the token from [the OS keyring or from ghtkn](docs/github_token.md).

5. Optionally, write a configuration file:

```sh
pinact init
```

The configuration file is optional. It says which files to pin, which actions to ignore, and what the default minimum release age is. See [Configuration File](docs/config.md).

## Installing the Agent Skill

pinact ships a single skill. It holds no documentation of its own: it tells the coding agent to read the documentation embedded in the pinact binary with `pinact docs list` and `pinact docs show <name>`, so the agent always reads the documentation of the version it is actually running.

[gh skill install](https://cli.github.com/manual/gh_skill_install):

```sh
gh skill install suzuki-shunsuke/pinact pinact
```

## Documentation

The documentation is split by topic under [`docs/`](docs). These documents are embedded in the pinact binary, so `pinact docs list` and `pinact docs show <name>` serve exactly what is listed below, matching the version that is installed. They are the single source of truth, shared between this README, the embedded documentation, and the skill, so there's no duplicated maintenance.

```sh
pinact docs list # The name and the description of every document, as JSON
pinact docs show config # One document
pinact docs show codes/005 # A document in a subdirectory is named by its path
```

- [Usage](docs/usage.md) - run pinact, which files it pins when none is given, and pinning actions written in a document.
- [Checking without fixing](docs/check.md) - `--check`, `--fix=false`, the offline check `--no-api`, and verifying version comments.
- [Updating actions](docs/update.md) - `--update` and the minimum release age (cooldown).
- [Pin branches](docs/branch_to_tag.md) - `--branch-to-tag`, which opts a branch reference in to being pinned.
- [Include and exclude specific actions](docs/include_exclude.md) - `--include` and `--exclude`.
- [SARIF](docs/sarif.md) - `--format sarif`, reviewdog, and GitHub code scanning.
- [Pin only changed lines](docs/diff_file.md) - `--diff-file`, to introduce pinact gradually.
- [GitHub access token](docs/github_token.md) - `PINACT_GITHUB_TOKEN`, the ghtkn integration, and the OS keyring.
- [Configuration File](docs/config.md) - the configuration file, the global configuration file, and every field of the schema.
- [GitHub Enterprise Server](docs/ghes.md) - pinning actions hosted on GHES.
- [Exit codes](docs/exit_codes.md) - what 0, 1, 2, and 3 mean.
- [Why doesn't pinact pin some actions?](docs/why_pinact_not_pin.md) - why a branch reference isn't pinned by default.
- [Verify version comments](docs/codes/001.md) - why a version comment isn't necessarily true, and how `--verify-comment` checks it.
- [SHA-pinned action requires a version comment](docs/codes/005.md) - why a bare SHA is rejected, and how to resolve it.
- [Schema version is required](docs/codes/002.md), [this version was abandoned](docs/codes/003.md), [unsupported configuration format version](docs/codes/004.md) - the configuration schema version errors.
- [Old schemas](docs/old_schema.md) - the configuration schema versions pinact no longer supports.
- [Upgrade guide: v3 to v4](docs/upgrade_guide/v4.md), [v4 to v5](docs/upgrade_guide/v5.md) - what changed between major versions.

[USAGE.md](USAGE.md) is the help of every command, generated from the CLI itself.

## GitHub Actions

https://github.com/suzuki-shunsuke/pinact-action

We develop GitHub Actions to pin GitHub Actions and reusable workflows by pinact.

## Motivation

It is a good manner to pin GitHub Actions versions by commit hash.
GitHub tags are mutable so they have a substantial security and reliability risk.

See also [Security hardening for GitHub Actions - GitHub Docs](https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#using-third-party-actions)

> Pinning an action to a full length commit SHA is currently the only way to use an action as an immutable release.
> Pinning to a particular SHA helps mitigate the risk of a bad actor adding a backdoor to the action's repository, as they would need to generate a SHA-1 collision for a valid Git object payload

:thumbsup:

```yaml
uses: actions/cache@88522ab9f39a2ea568f7027eddc7d8d8bc9d59c8 # v3.3.1
```

:thumbsdown:

```yaml
uses: actions/cache@v3
```

```yaml
uses: actions/cache@v3.3.1
```

## Why not using Renovate's helpers:pinGitHubActionDigestsToSemver preset?

The Renovate preset [helpers:pinGitHubActionDigestsToSemver](https://docs.renovatebot.com/presets-helpers/#helperspingithubactiondigeststosemver) is useful, but pinact is still useful:
You can use both the preset and pinact together.

1. Renovate can't pin actions in pull requests before merging them.
If you use linters such as [ghalint](https://github.com/suzuki-shunsuke/ghalint) in CI, you need to pin actions before merging pull requests
(ref. [ghalint policy to enforce actions to be pinned](https://github.com/suzuki-shunsuke/ghalint/blob/main/docs/policies/008.md))
2. Even if you use Renovate, sometimes you would want to update actions manually
3. pinact is useful for non Renovate users
4. [pinact supports verifying version annotations](docs/codes/001.md)

## See also

- [Renovate github-actions Manager - Additional Information](https://docs.renovatebot.com/modules/manager/github-actions/#additional-information)
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet) is a great tool, but there are [known issues](https://github.com/sethvargo/ratchet#known-issues).
