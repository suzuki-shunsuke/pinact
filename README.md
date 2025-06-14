# pinact

[Motivation](#motivation) | [Install](INSTALL.md) | [How to use](#how-to-use) | [GitHub Actions](https://github.com/suzuki-shunsuke/pinact-action) | [Configuration](#configuration) | [Contributing](CONTRIBUTING.md) | [LICENSE](LICENSE)

pinact is a CLI to edit GitHub Workflow and Composite action files and pin versions of Actions and Reusable Workflows.
pinact can also [update their versions](#update-actions), [verify version annotations](docs/codes/001.md), and [create reviews](#create-reviews).

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

Creating reviews:

![review](https://github.com/user-attachments/assets/77e78d23-bd14-49ba-8097-751556fcf126)

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
You can pass GitHub Access token via environment variable `GITHUB_TOKEN`.
If no GitHub Access token is passed, pinact calls GitHub REST API without access token.

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

### Fix example codes in documents

pinact can fix example codes in documents too.

```sh
pinact run README.md
```

### Create reviews

![review](https://github.com/user-attachments/assets/77e78d23-bd14-49ba-8097-751556fcf126)

As of pinact v3.3.0, pinact can create reviews by GitHub API.
A GitHub access token with `pull_requests:write` permission is required.

```sh
pinact run \
  -review \
  -repo-owner <repository owner> \
  -repo-name <repository name> \
  -pr <pull request number> \
  -sha <commit SHA to be reviewed>
```

If pinact is run via GitHub Actions `pull_request` event, options are auto-completed.

> [!WARNING]
> GitHub can't create pull request reviews on files not changed by the pull request.
> When pinact fails to create reviews, pinact outputs warning and creates [GitHub Actions error messages to log instead](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions#setting-an-error-message).
> You can ignore the warning like this:
> ```
> WARN[0004] create a review comment                       error="create a review comment: POST https://api.github.com/repos/szksh-lab-2/test-github-action/pulls/317/comments: 422 Validation Failed [{Resource:PullRequestReviewComment Field:pull_request_review_thread.path Code:invalid Message:} {Resource:PullRequestReviewComment Field:pull_request_review_thread.diff_hunk Code:missing_field Message:}]" line="      - uses: suzuki-shunsuke/watch-star-action@feat/first-pr" line_number=14 pinact_version=3.3.0-5 program=pinact review_pr_number=317 review_repo_name=test-github-action review_repo_owner=szksh-lab-2 review_sha=92f0b04efdc10acb793e78bdd1f70958dd3fd9a3 workflow_file=.github/workflows/watch.yaml
> ```

![error-message-log](https://github.com/user-attachments/assets/0231dee4-4473-459b-8ea4-e4c6a1f417c8)

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

`pinact >= v1.6.0` [#816](https://github.com/suzuki-shunsuke/pinact/pull/816)

Instead of fixing files, you can validate if actions are pinned by `--check` option:

```sh
pinact run --check
```

Using this option, pinact doesn't fix files.
If actions aren't pinned, the command fails.

```console
$ pinact run --check
ERRO[0000] parse a line                                  action=actions/checkout@v2 error="action isn't pinned" pinact_version= program=pinact workflow_file=testdata/foo.yaml
ERRO[0000] parse a line                                  action=actions/cache@v3.3.1 error="action isn't pinned" pinact_version= program=pinact workflow_file=testdata/foo.yaml
ERRO[0000] parse a line                                  action=rharkor/caching-for-turbo@v1.6 error="action isn't pinned" pinact_version= program=pinact workflow_file=testdata/foo.yaml
ERRO[0000] parse a line                                  action=actions/checkout@v3 error="action isn't pinned" pinact_version= program=pinact workflow_file=testdata/foo.yaml
ERRO[0000] parse a line                                  action=actions/checkout@v3 error="action isn't pinned" pinact_version= program=pinact workflow_file=testdata/foo.yaml
ERRO[0000] parse a line                                  action=suzuki-shunsuke/actionlint-workflow/.github/workflows/actionlint.yaml@v0.5.0 error="action isn't pinned" pinact_version= program=pinact workflow_file=testdata/foo.yaml

$ echo $?
1
```

If `-check` is set, files aren't fixed and no diff is outputted.
If you want to fix files, please use `-fix` option.

```sh
pinact run -check -fix
```

And if you want to output diff, please use `-diff` option.

```sh
pinact run -check -diff
```

### Verify version annotations

Please see [the document](docs/codes/001.md).

### Output diff

```console
$ pinact run -diff
INFO[0000] action isn't pinned
.github/workflows/test.yaml:8
-       - uses: actions/checkout@v4
+       - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
  pinact_version=v3.0.0-local program=pinact
```

### -diff, -check, -fix options

The behaviour of `pinact run` command is changed by command line options `-diff`, `-check`, and `-fix`.
This is a table how the behaviour is changed by these options.

options | Fix files | Exit with code 1 if actions aren't pinned | Output changes
--- | --- | --- | ---
No option | o | | |
-check | | o | |
-diff | | | o
-check -diff | | o | o
-check -fix | o | o | o
-fix -diff | o | | o

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
ignored_actions:
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
> Regular expressions must match with action names exactly.
> For instance, `ref: main` doesn't match with `malicious-main`

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

In some cases pinact doesn't pin versions intentionally, which may confuse you.
So we describe the reason here.

pinact doesn't pin actions whose versions aren't semver (e.g. `main`, `master`, `release/v1`).
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

## See also

- [Renovate github-actions Manager - Additional Information](https://docs.renovatebot.com/modules/manager/github-actions/#additional-information)
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet) is a great tool, but there are [known issues](https://github.com/sethvargo/ratchet#known-issues).

## LICENSE

[MIT](LICENSE)
