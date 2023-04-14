# pinact

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

```console
$ pinact help
NAME:
   pinact - Pin GitHub Actions versions. https://github/com/suzuki-shunsuke/pinact

USAGE:
   pinact [global options] command [command options] [arguments...]

VERSION:
   0.1.0 (8ccd55944c83ff1f4d738343c28d4a6109246d06)

COMMANDS:
   version  Show version
   run      Pin GitHub Actions versions
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --log-level value  log level [$PINACT_LOG_LEVEL]
   --help, -h         show help
   --version, -v      print the version
```

```console
$ pinact help run
NAME:
   pinact run - Pin GitHub Actions versions

USAGE:
   pinact run [command options] [arguments...]

DESCRIPTION:
   If no argument is passed, pinact searches GitHub Actions workflow files from .github/workflows.

   $ pinact run

   You can also pass workflow file paths as arguments.

   e.g.

   $ pinact run .github/actions/foo/action.yaml .github/actions/bar/action.yaml


OPTIONS:
   --help, -h  show help
```

## See also

- [Renovate - helpers:pinGitHubActionDigests](https://docs.renovatebot.com/presets-helpers/#helperspingithubactiondigests): This is useful, but this doesn't change the short format tag such as `v2` to the long format tag such as `v2.0.0`
- [sethvargo/ratchet](https://github.com/sethvargo/ratchet): This is a great tool, but there are [known issues](https://github.com/sethvargo/ratchet#known-issues)

## LICENSE

[MIT](LICENSE)
