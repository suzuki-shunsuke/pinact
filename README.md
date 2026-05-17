# pinact

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/suzuki-shunsuke/pinact)
[Install](INSTALL.md) | [How to use](#how-to-use) | [Configuration](#configuration)

pinact is a CLI to pin GitHub Actions and Reusable Workflows.
pinact can also [update their versions](#update-actions) and [verify version comments](docs/codes/001.md).

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
1. [Check if actions are pinned without editing files](#just-validation--check--fixfalse)
1. [Offline check without GitHub API](#offline-check--no-api)
1. [Update actions](#update-actions--update) with a [minimum release age](#minimum-release-age-cooldown--min-age)
1. [Verify version comments](docs/codes/001.md) ([`-verify-comment`](#verify-version-comments--verify-comment--verify--v))
1. [Verify if actions meet the minimum release age](#minimum-release-age-cooldown--min-age)
1. [Pin branches](#pin-branches--branch-to-tag)
1. [Include and exclude specific actions](#include-and-exclude-specific-actions)
1. [Generate SARIF](#sarif). This is useful to create reviews using [reviewdog](#reviewdog)
1. [Read GitHub access token via keyrings or ghtkn](#github-access-token)
1. [Support GitHub Enterprise Server](docs/ghes.md)
1. [GitHub Action](https://github.com/suzuki-shunsuke/pinact-action)

## Usage

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

[pinact calls GitHub API to fetch releases and tags. To avoid api rate limiting, you should pass a GitHub Access token.](#github-access-token)

### Fix example codes in documents

Not only workflow files, but also text files of any formats are supported.
This is useful to pin actions in text files such as `README.md`.

```sh
pinact run README.md
```

### Just Validation: `-check`, `-fix=false`

By default, pinact edit files.
If `-check` or `-fix=false` is specified, pinact just checks if actions are pinned without editing files.

```sh
pinact run -check
```

### Offline Check: `-no-api`

For an offline check (no GitHub API call, only the 40-character SHA syntactic check), add `-no-api`:

```sh
pinact run -fix=false -no-api
```

With `-no-api`, pinact can't fetch action versions and SHA, so pinact can't pin actions.
So it only checks if actions are pinned with full-length commit SHA.

### Update Actions: `-update`

Update actions to latest versions:

```sh
pinact run -update
```

### Minimum Release Age (Cooldown): `-min-age`, `-verify-min-age`

pinact supports two kinds of minimum release age checks:

1. Verify current versions: Verify if current action versions meet the minimum release age requirement
1. Verify new versions: Exclude versions that don't meet the minimum release age requirement when updating actions (`-update`)
    1. If no release meeting the given minimum age is found, pinact will exit with an error.

This helps reduce supply chain security risks.

By default, no minimum release age is set.
You can set the minimum release age by some methods:

1. `-min-age <minimum release age>`: Set the minimum release age in days

```sh
pinact run -min-age 7
```

2. Environment variable `PINACT_MIN_AGE`
3. Configuration file `.pinact.yml`
    1. `.rules[].min_age`: A rule specific minimum release age in days
    1. `.min_age.value`: The default minimum release age in days

```yaml
min_age:
  value: 7
rules:
  - min_age: 0
    conditions:
      - expr: |
          ActionRepoOwner == "suzuki-shunsuke"
```

It may be wasteful to verify all current versions against the minimum release age every time pinact runs.
Therefore, current versions are verified using the min_age setting in .pinact.yml and `PINACT_MIN_AGE` only when --verify-min-age is set or .min_age.always is true.

```sh
pinact run -verify-min-age
```

Or

```yaml
min_age:
  value: 7
  always: true # default is false
```

On the other hand, when updating actions min_age setting is always applied.

- For GitHub Releases, the `PublishedAt` date is checked
- For tags, the commit's `Committer.Date` is checked (requires additional API call)

### Verify Version Comments: `-verify-comment` (`-verify`, `-v`)

[Please see `Verify version comments`.](docs/codes/001.md)

```sh
pinact run -verify-comment
```

### Pin Branches: `-branch-to-tag`

pinact >= v3.10.0, [#1529](https://github.com/suzuki-shunsuke/pinact/issues/1529)

By default, pinact doesn't pin branches such as `main` or `master`.
If you want to pin specific branches, you can use the `--branch-to-tag` option.

```sh
pinact run --branch-to-tag '<regular expression matching branch name>'
```

The value is evaluated as a regular expression with partial match, just like `--include` / `--exclude`.
Anchor with `^...$` for an exact match - for short branch names like `main` this is recommended to avoid matching `mainline` etc.
Versions that don't match any of the supplied regexps continue to error out as before.

The branch is converted to the **latest stable tag** of the action. Pre-releases are used only when no stable tag exists.

[`--min-age`](#skip-recently-released-versions) is honored: when set, tags released within the cooldown window are skipped.

`--branch-to-tag` can be specified multiple times.

e.g.

```sh
pinact run --branch-to-tag '^main$' --branch-to-tag '^release/.*$'
```

### Include and exclude specific actions

[#1082](https://github.com/suzuki-shunsuke/pinact/pull/1082) pinact >= v3.4.0

You can fix only specific actions using the `-include (-i) <regular expression>` option.
You can also exclude only specific actions using the `-exclude (-e) <regular expression>` option.

e.g.

```sh
pinact run -i "actions/.*" -i "^aquaproj/aqua-installer$"
```

```sh
pinact run -e "actions/.*" -e "^aquaproj/aqua-installer$"
```

### SARIF

pinact >= v3.7.0 [#1294](https://github.com/suzuki-shunsuke/pinact/pull/1294)

pinact can output the result in [the SARIF format](https://sarifweb.azurewebsites.net/).

```sh
pinact run --format sarif
```

This format is useful to integration tools like [reviewdog](https://github.com/reviewdog/reviewdog) and [GitHub SARIF Code Scanning](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning).

`-format sarif` implies `-fix=false`, so files are not modified.
If you want to fix files, use `-fix`.

```sh
pinact run --format sarif -fix
```

#### Reviewdog

```sh
pinact run -format sarif |
  reviewdog -f sarif -name pinact -reporter github-pr-review
```

#### GitHub SARIF Code Scanning

```yaml
- run: pinact run -format sarif > sarif.json || true
- name: Upload SARIF file
  uses: github/codeql-action/upload-sarif@5d4e8d1aca955e8d8589aabd499c5cae939e33c7 # v4.31.9
  with:
    sarif_file: sarif.json
    category: pinact
```

## GitHub Access token

pinact calls GitHub REST API to get commit hashes and tags.
You can pass GitHub Access token via environment variable `PINACT_GITHUB_TOKEN` or `GITHUB_TOKEN`.
If no GitHub Access token is passed, pinact calls GitHub REST API without access token.
About GitHub Enterprise Server, see also [GitHub Access Token for GHES](#github-access-token-for-ghes).

### Manage GitHub Access token using ghtkn

pinact >= v3.8.0

[You can create a GitHub App User Access Token by ghtkn integration](https://github.com/suzuki-shunsuke/ghtkn).
About ghtkn, please see the document of ghtkn.
You need to set up ghtkn first.

```sh
export PINACT_GHTKN=true
```

### Manage GitHub Access token using Keyring

pinact >= v3.1.0

You can manage a GitHub Access token using secret store such as [Windows Credential Manager](https://support.microsoft.com/en-us/windows/accessing-credential-manager-1b5c916a-6a16-889f-8581-fc16e8165ac0), [macOS Keychain](https://en.wikipedia.org/wiki/Keychain_(software)), and [GNOME Keyring](https://wiki.gnome.org/Projects/GnomeKeyring).

1. Configure a GitHub Access token by `pinact token set` command:

```console
$ pinact token set
Enter a GitHub access token: # Input GitHub Access token
```

or you can also pass a GitHub Access token via standard input:

```sh
echo "<github access token>" | pinact token set -stdin
```

2. Enable the feature by setting the environment variable `PINACT_KEYRING_ENABLED`:

```sh
export PINACT_KEYRING_ENABLED=true
```

Note that if the environment variable `GITHUB_TOKEN` is set, this feature gets disabled.

You can remove a GitHub Access token from keyring by `pinact token rm` command:

```sh
pinact token rm
```

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Everything is pinned, or pinact fixed it |
| 1 | `-fix=false` was set and something needs pinning |
| 2 | An action cannot be auto-fixed (branch reference, `-verify-comment` mismatch, or `-min-age` violation) |
| 3 | GitHub API error, invalid CLI flag combination, or other unexpected error |

## GitHub Actions

https://github.com/suzuki-shunsuke/pinact-action

We develop GitHub Actions to pin GitHub Actions and reusable workflows by pinact.

## Configuration File

[JSON Schema](json-schema/pinact.json)

A configuration file is optional.
pinact supports a configuration file `.pinact.yaml`, `.github/pinact.yaml`, `.pinact.yml` or `.github/pinact.yml`.
You can also specify the configuration file path by the environment variable `PINACT_CONFIG` or command line option `-c`.
You can generate a configuration file by `pinact init`.

```sh
pinact init [<configuration file path>]
```

For more details, see [Configuration File](docs/config.md).

## Q. Why doesn't pinact pin some actions?

> [!TIP]
> Since v3.10.0, the [`--branch-to-tag`](#pin-branches) option lets you opt-in to pinning specific branches to the latest stable tag of an action.

In some cases pinact doesn't pin versions intentionally, which may confuse you.
For instance, pinact doesn't pin branches like `main` and `master` by default.
[For more details, please see here?](docs/why-pinact-not-pin.md).

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
4. [pinact supports verifying version annotations](https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/001.md)

## See also

- [Renovate github-actions Manager - Additional Information](https://docs.renovatebot.com/modules/manager/github-actions/#additional-information)
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet) is a great tool, but there are [known issues](https://github.com/sethvargo/ratchet#known-issues).
