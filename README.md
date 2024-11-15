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

[pinact also supports verifying version annotations](docs/codes/001.md).

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

## Install

pinact is written in Go. So you only have to install a binary in your `PATH`.

There are some ways to install pinact.

1. [Homebrew](#homebrew)
1. [aqua](#aqua)
1. [GitHub Releases](#github-releases)
1. [Build an executable binary from source code yourself using Go](#build)

### Homebrew

You can install pinact using [Homebrew](https://brew.sh/).

```console
$ brew install suzuki-shunsuke/pinact/pinact
```

### aqua

`aqua-registry >= v3.154.0`

You can install pinact using [aqua](https://aquaproj.github.io/).

```console
$ aqua g -i suzuki-shunsuke/pinact
```

### GitHub Releases

You can download an asset from [GitHub Reelases](https://github.com/suzuki-shunsuke/pinact/releases).
Please unarchive it and install a pre built binary into `$PATH`. 

<details>
<summary>Verify downloaded assets from GitHub Releases</summary>

You can verify downloaded assets using some tools.

1. [GitHub CLI](https://cli.github.com/)
1. [slsa-verifier](https://github.com/slsa-framework/slsa-verifier)
1. [Cosign](https://github.com/sigstore/cosign)

#### 1. GitHub CLI

pinact >= v1.0.0

You can install GitHub CLI by aqua.

```sh
aqua g -i cli/cli
```

```sh
gh release download -R suzuki-shunsuke/pinact v1.0.0 -p pinact_darwin_arm64.tar.gz
gh attestation verify pinact_darwin_arm64.tar.gz \
  -R suzuki-shunsuke/pinact \
  --signer-workflow suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml
```

Output:

```
Loaded digest sha256:73d06ea7c7be9965c47863b2d9c04f298ae1d37edc18e162c540acb4ac030314 for file://pinact_darwin_arm64.tar.gz
Loaded 1 attestation from GitHub API
✓ Verification succeeded!

sha256:73d06ea7c7be9965c47863b2d9c04f298ae1d37edc18e162c540acb4ac030314 was attested by:
REPO                                 PREDICATE_TYPE                  WORKFLOW
suzuki-shunsuke/go-release-workflow  https://slsa.dev/provenance/v1  .github/workflows/release.yaml@7f97a226912ee2978126019b1e95311d7d15c97a
```

#### 2. slsa-verifier

You can install slsa-verifier by aqua.

```sh
aqua g -i slsa-framework/slsa-verifier
```

```sh
gh release download -R suzuki-shunsuke/pinact v1.0.0
slsa-verifier verify-artifact pinact_darwin_arm64.tar.gz \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/suzuki-shunsuke/pinact \
  --source-tag v1.0.0
```

Output:

```
Verified signature against tlog entry index 136997022 at URL: https://rekor.sigstore.dev/api/v1/log/entries/108e9186e8c5677ae52579043db716d86ec00ccb03b2032503dc3a5a88d9e8b60c48c3d1cf62c5d1
Verified build using builder "https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@refs/tags/v2.0.0" at commit 664dfa3048ab7e48a581538740ea9698002703cd
Verifying artifact pinact_darwin_arm64.tar.gz: PASSED

PASSED: SLSA verification passed
```

#### 3. Cosign

You can install Cosign by aqua.

```sh
aqua g -i sigstore/cosign
```

```sh
gh release download -R suzuki-shunsuke/pinact v1.0.0
cosign verify-blob \
  --signature pinact_1.0.0_checksums.txt.sig \
  --certificate pinact_1.0.0_checksums.txt.pem \
  --certificate-identity-regexp 'https://github\.com/suzuki-shunsuke/go-release-workflow/\.github/workflows/release\.yaml@.*' \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  pinact_1.0.0_checksums.txt
```

Output:

```
Verified OK
```

After verifying the checksum, verify the artifact.

```sh
cat pinact_1.0.0_checksums.txt | sha256sum -c --ignore-missing
```

</details>

### Build an executable binary from source code yourself using Go

```sh
go install github.com/suzuki-shunsuke/pinact/cmd/pinact@latest
```

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
You can create a configuration file `.pinact.yaml` by `pinact init`.

```console
$ pinact init
```

You can change the output path.

```console
$ pinact init '.github/pinact.yaml'
```

About the configuration, please see [Configuration](#Configuration).

## Verify version annotations

Please see [the document](docs/codes/001.md).

## GitHub Actions

https://github.com/suzuki-shunsuke/pinact-action

We develop GitHub Actions to pin GitHub Actions and reusable workflows by pinact.

## Configuration

pinact supports a configuration file `.pinact.yaml` or `.github/pinact.yaml`.
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
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet) is a great tool, but there are [known issues](https://github.com/sethvargo/ratchet#known-issues).

## LICENSE

[MIT](LICENSE)
