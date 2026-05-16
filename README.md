# pinact

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/suzuki-shunsuke/pinact)
[Install](INSTALL.md) | [How to use](#how-to-use) | [Configuration](#configuration)

pinact is a CLI to edit GitHub Workflow and Composite action files and pin versions of Actions and Reusable Workflows.
pinact can also [update their versions](#update-actions) and [verify version annotations](docs/codes/001.md).

```sh
pinact run
```

```diff
$ git diff
diff --git a/.github/workflows/test.yaml b/.github/workflows/test.yaml
index 84bd67a..5d92e44 100644
--- a/.github/workflows/test.yaml
+++ b/.github/workflows/test.yaml
@@ -113,17 +113,17 @@ jobs:
     needs: path-filter
     permissions: {}
     steps:
-      - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3
-      - uses: actions/setup-go@v4
+      - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1
+      - uses: actions/setup-go@4d34df0c2316fe8122ab82dc22947d607c0c91f9 # v4.0.0
       - name: Cache Primes
         id: cache-primes
-        uses: actions/cache@v3.3.1
+        uses: actions/cache@88522ab9f39a2ea568f7027eddc7d8d8bc9d59c8 # v3.3.1
         with:
           path: prime-numbers
           key: ${{ runner.os }}-primes

   actionlint:
-    uses: suzuki-shunsuke/actionlint-workflow/.github/workflows/actionlint.yaml@v0.5.0
+    uses: suzuki-shunsuke/actionlint-workflow/.github/workflows/actionlint.yaml@b6a5f966d4504893b2aeb60cf2b0de8946e48504 # v0.5.0
     with:
       aqua_version: v2.3.4
     permissions:
```

## Migrating from v3 to v4

Most v3 invocations keep working in v4 unchanged — `--check`, `--diff`, `--verify`, and `-v` are kept as silent aliases. Only the `-review` family is removed; the recommended replacement is SARIF output piped to [reviewdog](#reviewdog). See [spec.md](spec.md#v3--v4-移行ガイド) and [#1538](https://github.com/suzuki-shunsuke/pinact/issues/1538) for the full migration guide.

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

1. Renovate can't pin actions in pull requests before merging them.
If you use linters such as [ghalint](https://github.com/suzuki-shunsuke/ghalint) in CI, you need to pin actions before merging pull requests
(ref. [ghalint policy to enforce actions to be pinned](https://github.com/suzuki-shunsuke/ghalint/blob/main/docs/policies/008.md))
2. Even if you use Renovate, sometimes you would want to update actions manually
3. pinact is useful for non Renovate users
4. [pinact supports verifying version annotations](https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/001.md)

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

## How to use

Please run `pinact run` on a Git repository root directory, then target files are fixed.

```sh
pinact run
```

Default target files are:

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

You can change target files by command line arguments or configuration files.

e.g.

```sh
pinact run example.yaml
```

### Update actions

[#663](https://github.com/suzuki-shunsuke/pinact/pull/663) pinact >= v1.1.0

You can update actions using the `-update (-u)` option:

```sh
pinact run -u
```

#### Skip recently released versions

[#1266](https://github.com/suzuki-shunsuke/pinact/pull/1266) pinact >= v3.5.0

You can skip recently released versions using the `--min-age` (`-m`) option or the environment variable `PINACT_MIN_AGE`.
This helps avoid updating to potentially unstable versions that haven't had time to prove their stability.

```sh
pinact run -u --min-age 7
```

or

```sh
export PINACT_MIN_AGE=7
pinact run -u
```

This command skips versions released within the last 7 days.

- For GitHub Releases, the `PublishedAt` date is checked
- For tags, the commit's `Committer.Date` is checked (requires additional API call)

### Fix example codes in documents

pinact can fix example codes in documents too.

```sh
pinact run README.md
```

### Generate a configuration file `.pinact.yaml`

A configuration file is optional.
You can create a configuration file `.pinact.yaml` by `pinact init`.

```sh
pinact init
```

You can change the output path.

```sh
pinact init '.github/pinact.yaml'
```

About the configuration, please see [Configuration](#Configuration).

### Validation

Validate that all actions are pinned without modifying any files:

```sh
pinact run -fix=false
```

`-fix=false` keeps the files as-is and exits with code 1 when an action needs pinning. The detailed diff is still printed to stderr so you can see what would change.

```console
$ pinact run -fix=false
.github/workflows/test.yaml:8
- - uses: actions/checkout@v4
+ - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4.3.1

$ echo $?
1
```

For an offline check (no GitHub API call, only the 40-character SHA syntactic check), add `-no-api`:

```sh
pinact run -fix=false -no-api
```

`--check` continues to work as a silent alias for `-fix=false` in v4.

### Verify version annotations

Please see [the document](docs/codes/001.md).

### Output diff

The line-by-line diff is always printed to stderr — running `pinact run` with no options is enough:

```console
$ pinact run
.github/workflows/test.yaml:8
- - uses: actions/checkout@v4
+ - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4.3.1
```

By default `pinact run` also writes the fix to disk. Use `-fix=false` to preview without writing.

### Pin branches

pinact >= v3.10.0, [#1529](https://github.com/suzuki-shunsuke/pinact/issues/1529)

By default, pinact doesn't pin branches such as `main` or `master`.
If you want to pin specific branches, you can use the `--branch-to-tag` option.

```sh
pinact run --branch-to-tag '<regular expression matching branch name>'
```

The value is evaluated as a regular expression with partial match, just like `--include` / `--exclude`.
Anchor with `^...$` for an exact match — for short branch names like `main` this is recommended to avoid matching `mainline` etc.
Versions that don't match any of the supplied regexps continue to error out as before.

The branch is converted to the **latest stable tag** of the action. Pre-releases are used only when no stable tag exists.

[`--min-age`](#skip-recently-released-versions) is honored: when set, tags released within the cooldown window are skipped.

`--branch-to-tag` can be specified multiple times.

e.g.

```sh
pinact run --branch-to-tag '^main$' --branch-to-tag '^release/.*$'
```

### `-fix`, `-no-api`, `-format sarif`

The behaviour of `pinact run` is controlled by these orthogonal options.

Default behaviour:

- Fix files (`-fix=true`)
- Always print a line-by-line diff (or `file:line + line` for non-fixable actions) to stderr
- Call the GitHub API to resolve SHAs

Each option:

- `-fix=false` — don't write files. Exits with code 1 if there is something to pin.
- `-no-api` — don't call the GitHub API. Only the syntactic SHA check is performed; combine with `-fix=false` or `-format sarif`.
- `-format sarif` — write a SARIF report to stdout. Implies `-fix=false` unless `-fix` is also passed.
- `-verify-comment` — verify that the SHA matches its version comment (e.g. `@<sha> # v1.2.3`).

v3 aliases (still accepted, no deprecation warning):

- `--check` → `-fix=false`
- `--diff` → `-fix=false` (note: `-diff=false` is ignored because the diff is always printed; a warning is emitted)
- `--verify`, `-v` → `-verify-comment`

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Everything is pinned, or pinact fixed it |
| 1 | `-fix=false` was set and something needs pinning |
| 2 | An action cannot be auto-fixed (branch reference, `-verify-comment` mismatch, or `-min-age` violation) |
| 3 | GitHub API error, invalid CLI flag combination, or other unexpected error |

## Fix or exclude only specific actions

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

## SARIF

pinact >= v3.7.0 [#1294](https://github.com/suzuki-shunsuke/pinact/pull/1294)

pinact can output the result in [the SARIF format](https://sarifweb.azurewebsites.net/).

```sh
pinact run --format sarif
```

This format is useful to integration tools like [reviewdog](https://github.com/reviewdog/reviewdog) and [GitHub SARIF Code Scanning](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning).

### Reviewdog

`-format sarif` implies `-fix=false`, so files are not modified.

```sh
pinact run -format sarif |
  reviewdog -f sarif -name pinact -reporter github-pr-review
```

### GitHub SARIF Code Scanning

```yaml
- run: pinact run -format sarif > sarif.json || true
- name: Upload SARIF file
  uses: github/codeql-action/upload-sarif@5d4e8d1aca955e8d8589aabd499c5cae939e33c7 # v4.31.9
  with:
    sarif_file: sarif.json
    category: pinact
```

## GitHub Actions

https://github.com/suzuki-shunsuke/pinact-action

We develop GitHub Actions to pin GitHub Actions and reusable workflows by pinact.

## Configuration

A configuration file is optional.
pinact supports a configuration file `.pinact.yaml`, `.github/pinact.yaml`, `.pinact.yml` or `.github/pinact.yml`.
You can also specify the configuration file path by the environment variable `PINACT_CONFIG` or command line option `-c`.

As of pinact v2.2.0, pinact configuration file has a schema version.

```yaml
version: 3
```

In general, you should use the latest schema version.

### Schema v3 (latest)

pinact v2.2.0 or later supports this version.

.pinact.yaml

e.g.

```yaml
version: 3
files:
  - pattern: .github/workflows/*.yml
  - pattern: .github/workflows/*.yaml
  - pattern: .github/actions/*/action.yml
  - pattern: .github/actions/*/action.yaml

ignore_actions:
  # slsa-framework/slsa-github-generator doesn't support pinning version
  # > Invalid ref: 68bad40844440577b33778c9f29077a3388838e9. Expected ref of the form refs/tags/vX.Y.Z
  # https://github.com/slsa-framework/slsa-github-generator/issues/722
  - name: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml
    ref: "v\\d+\\.\\d+\\.\\d+"
  - name: suzuki-shunsuke/.*
    ref: main

# GitHub Enterprise Server Support
ghes:
  api_url: https://ghes.example.com
  fallback: true # optional, default is false

# Separator between version and tag comment (optional, default is " # ")
# pinact >= v3.9.0
separator: " # "
```

#### `files`

This is optional.
A list of target files.

#### `files[].pattern`

This is required.
A glob pattern of target files.
[Go's path/filepath#Glob](https://pkg.go.dev/path/filepath#Glob) is used.
A relative path from pinact's configuration file.
If files are passed via positional command line arguments, the configuration is ignored.

e.g.

```yaml
files:
  - pattern: .github/workflows/*.yml
  - pattern: .github/workflows/*.yaml
  - pattern: README.md
```

#### `ignore_actions`

This is optional. A list of ignored actions and reusable workflows.

#### `ignore_actions[].name`

This is required.
A regular expression of ignored actions and reusable workflows.

```yaml
ignore_actions:
  - name: actions/.*
    ref: main
```

> [!WARNING]
> Regular expressions must match with action names exactly.
> For instance, `name: actions/` doesn't match with `actions/checkout`

Regarding regular expressions, [Go's regexp package is used.](https://pkg.go.dev/regexp)

#### `ignore_actions[].ref`

This is required.
A regular expression of ignored action versions (branch, tag, or commit hash).

> [!WARNING]
> Regular expressions must match with action versions exactly.
> For instance, `ref: main` doesn't match with `malicious-main`

#### `ghes`

[See GitHub Enterprise Support](#github-enterprise-server-ghes-support).

#### `separator`

pinact >= v3.9.0 [#1365](https://github.com/suzuki-shunsuke/pinact/pull/1365) [#1372](https://github.com/suzuki-shunsuke/pinact/pull/1372)

This is optional. Default is ` # `.
The separator between the action version (commit SHA) and the version tag comment.
It must include `#`.
You can also configure the separator by command line option `--separator (-sep)` or environment variable `PINACT_SEPARATOR`.

e.g.

```yaml
# Default separator " # "
separator: " # "
# Results in: uses: actions/checkout@abc123... # v3.5.0

# Custom separator " # tag="
separator: " # tag="
# Results in: uses: actions/checkout@abc123... # tag=v3.5.0

# Custom separator with double space before #
separator: "  # "
# Results in: uses: actions/checkout@abc123...  # v3.5.0
```

### Old Schemas

Please see [here](docs/old_schema.md).

### JSON Schema

- [pinact.json](json-schema/pinact.json)
- https://raw.githubusercontent.com/suzuki-shunsuke/pinact/refs/heads/main/json-schema/pinact.json

If you look for a CLI tool to validate configuration with JSON Schema, [ajv-cli](https://ajv.js.org/packages/ajv-cli.html) is useful.

```sh
ajv --spec=draft2020 -s json-schema/pinact.json -d pinact.yaml
```

#### Input Complementation by YAML Language Server

[Please see the comment too.](https://github.com/szksh-lab/.github/issues/67#issuecomment-2564960491)

Version: `main`

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/suzuki-shunsuke/pinact/main/json-schema/pinact.json
```

Or pinning version:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/suzuki-shunsuke/pinact/v1.1.2/json-schema/pinact.json
```

## Q. Why doesn't pinact pin some actions?

> [!TIP]
> Since v3.10.0, the [`--branch-to-tag`](#pin-branches) option lets you opt-in to pinning specific branches to the latest stable tag of an action.

In some cases pinact doesn't pin versions intentionally, which may confuse you.
So we describe the reason here.

By default, pinact doesn't pin actions whose versions aren't semver (e.g. `main`, `master`, `release/v1`).
This is because pinact is designed as a safe tool so that it doesn't change workflows behaviour.
pinact pins actions but doesn't change SHA of actions at the moment when pinact pins versions.

This design enables you to accept changes by pinact safely.

For instance, pinact changes the version `v1` to `v1.1.0` if their SHA are equivalent.
If there are no semver whose SHA is same with `v1`, pinact doesn't change the version.

And pinact doesn't change versions which aren't semver.
For instance, pinact doesn't change the version `main`.

```yaml
uses: actions/checkout@main
```

We don't want to pin `main` to full commit length SHA like the following because we can't update this following semantic versioning.

```yaml
uses: actions/checkout@85e6279cec87321a52edac9c87bce653a07cf6c2 # main
```

Tools like Renovate can update the SHA, but it's not safe at all as `main` branch isn't stable.

And we don't want to change `main` to the latest semver like the following because SHA is changed and workflows may be broken.

```yaml
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # 4.2.2
```

We don't want to pin branches as SHA of branches is changed.

pinact doesn't check if a version is a tag or a branch because we would like to reduce the number of API calls as much as possible.
If a version isn't semver, pinact judges it may be a branch so pinact doesn't pin it.

Please see also [#926](https://github.com/suzuki-shunsuke/pinact/issues/926).

## GitHub Enterprise Server (GHES) Support

v3.6.0 [#839](https://github.com/suzuki-shunsuke/pinact/issues/839) [#1275](https://github.com/suzuki-shunsuke/pinact/pull/1275)

pinact also supports pinning versions of GitHub Actions hosted on GitHub Enterprise Server (GHES).
If the GHES support is enabled, pinact searches actions in GHES.

### Fallback to github.com

The fallback to github.com is disabled by default.
All actions are searched on the GHES instance only.
If the fallback is enabled, repositories of actions are first searched on the GHES instance. If repositoires are not found (404), pinact falls back to github.com. This is suitable when [GitHub Connect is enabled](https://docs.github.com/en/enterprise-server@3.19/admin/managing-github-actions-for-your-enterprise/managing-access-to-actions-from-githubcom/enabling-automatic-access-to-githubcom-actions-using-github-connect).

### GitHub Access Token for GHES

Set a GitHub Access Token for GHES using one of the following environment variables (checked in order):

1. `PINACT_GHES_TOKEN`
2. `GHES_TOKEN`
3. `GITHUB_TOKEN_ENTERPRISE`
4. `GITHUB_ENTERPRISE_TOKEN`

```sh
export GHES_TOKEN=xxx
```

`GITHUB_TOKEN` is used for github.com.

### Configuration File For GHES

GHES configuration is required via configuration file or environment variables.
The configuration file takes precedence over the environment variables.

```yaml
ghes:
  api_url: https://ghes.example.com
  fallback: true # optional, default is false
```

- `api_url`: API URL of the GHES instance. Can also be set via environment variables.
- `fallback`: Whether to fallback to github.com when a repository is not found on GHES. Default is `false`.

### Environment Variables For GHES

You can also configure GHES using environment variables instead of a configuration file.

- `GHES_API_URL`
- `PINACT_GHES_FALLBACK`

```sh
export GHES_API_URL=https://ghes.example.com
export PINACT_GHES_FALLBACK=true
```

If `GHES_API_URL` is not set, `GITHUB_API_URL` will be used instead.
This is convenient when running on GitHub Actions hosted on GHES.

### Conditions for Enabling GHES

GHES mode is enabled when any of the following conditions are met:

1. `ghes.api_url` is configured in the configuration file
2. `GHES_API_URL` environment variable is set
3. `GITHUB_API_URL` environment variable is set and is not `https://api.github.com`

## See also

- [Renovate github-actions Manager - Additional Information](https://docs.renovatebot.com/modules/manager/github-actions/#additional-information)
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet) is a great tool, but there are [known issues](https://github.com/sethvargo/ratchet#known-issues).

## LICENSE

[MIT](LICENSE)
