# pinact

[Motivation](#motivation) | [Install](#install) | [How to use](#how-to-use) | [GitHub Actions](https://github.com/suzuki-shunsuke/pinact-action) | [Configuration](#configuration) | [LICENSE](LICENSE)

Pin GitHub Actions versions

pinact edits GitHub Workflow files and pins versions of Actions and Reusable Workflows.

```console
$ pinact run
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

## Motivation

It is a good manner to pin GitHub Actions versions by commit hash.
GitHub tags are mutable so they have a substantial security and reliability risk.

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

[Renovate's helpers:pinGitHubActionDigests](https://docs.renovatebot.com/presets-helpers/#helperspingithubactiondigests) pins GitHub Actions versions by commit hash, but this doesn't change the short format tag such as `v2` to the long format tag such as `v2.0.0`.

```yaml
uses: actions/cache@88522ab9f39a2ea568f7027eddc7d8d8bc9d59c8 # v3
```

Even if the tag is short Renovate will update the commit hash, but you can't understand what is changed from the Renovate's pull request.

e.g. https://github.com/suzuki-shunsuke/test-github-action/pull/141

<img width="937" alt="image" src="https://user-images.githubusercontent.com/13323303/231947080-75930df5-a471-4b1a-a5ab-de5a270d7738.png">

pinact converts short tags to long tags, so you can understand what is changed from the Renovate's pull request.

```yaml
uses: actions/cache@88522ab9f39a2ea568f7027eddc7d8d8bc9d59c8 # v3.5.1
```

e.g. https://github.com/suzuki-shunsuke/test-github-action/pull/143

<img width="947" alt="image" src="https://user-images.githubusercontent.com/13323303/231948517-01fcbf19-9f6d-467a-9bb5-7cba097f2233.png">

## Install

pinact is written in Go. So you only have to install a binary in your `PATH`.

There are some ways to install pinact.

1. Homebrew: `brew install suzuki-shunsuke/pinact/pinact`
1. [aqua](https://aquaproj.github.io/): `aqua g -i suzuki-shunsuke/pinact` (`aqua-registry >= v3.154.0`)
1. Download a pre built binary from GitHub Releases
1. Build yourself with Go: `go install github.com/suzuki-shunsuke/pinact/cmd/pinact@latest`

## GitHub Access token

pinact calls GitHub REST API to get commit hashes and tags.
You can pass GitHub Access token via environment variable `GITHUB_TOKEN`.
If no GitHub Access token is passed, pinact calls GitHub REST API without access token.

## Usage

Please see [USAGE](USAGE.md).

## How to use

Please run `pinact run` on a Git repository root directory, then target files are fixed.

```console
$ pinact run
```

Default target files are `\.github/workflows/.*\.ya?ml$`, but you can change target files by command line arguments or configuration files.

e.g.

```console
$ pinact run action.yaml
```

A configuration file is optional.
You can create a configuration file by `pinact init`.

```console
$ pinact init
```

About the configuration, please see [Configuration](#Configuration).

## GitHub Actions

https://github.com/suzuki-shunsuke/pinact-action

We develop GitHub Actions to pin GitHub Actions and reusable workflows by pinact.

## Configuration

pinact supports a configuration file `.pinact.yaml`.
You can also specify the configuration file path by the environment variable `PINACT_CONFIG` or command line option `-c`.

.pinact.yaml

e.g.

```yaml
files:
  - pattern: "^\\.github/workflows/.*\\.ya?ml$"
  - pattern: "^(.*/)?action\\.ya?ml$"

ignore_actions:
  # slsa-framework/slsa-github-generator doesn't support pinning version
  # > Invalid ref: 68bad40844440577b33778c9f29077a3388838e9. Expected ref of the form refs/tags/vX.Y.Z
  # https://github.com/slsa-framework/slsa-github-generator/issues/722
  - name: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml
```

### `files[].pattern`

The regular expression of target files. If files are passed via positional command line arguments, the configuration is ignored.

### `ignore_actions[].name`

Action and reusable workflow names that pinact ignores.

## See also

- [Renovate github-actions Manager - Additional Information](https://docs.renovatebot.com/modules/manager/github-actions/#additional-information)
- [Renovate - helpers:pinGitHubActionDigests](https://docs.renovatebot.com/presets-helpers/#helperspingithubactiondigests): This is useful, but this doesn't change the short format tag such as `v2` to the long format tag such as `v2.0.0`
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet): This is a great tool, but there are [known issues](https://github.com/sethvargo/ratchet#known-issues)

## LICENSE

[MIT](LICENSE)
