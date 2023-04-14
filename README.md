# pinact

Pin GitHub Actions versions

```console
$ pinact run
```

```diff
$ git diff
diff --git a/.github/workflows/test.yaml b/.github/workflows/test.yaml
index 8161b12..08a9355 100644
--- a/.github/workflows/test.yaml
+++ b/.github/workflows/test.yaml
@@ -13,6 +13,7 @@ jobs:
     runs-on: ubuntu-latest
     permissions: {}
     steps:
-      - uses: actions/checkout@v2
+      - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0
       - uses: dorny/paths-filter@4512585405083f25c027a35db413c2b3b9006d50 # v2.11.1
         id: changes
         with:
@@ -62,6 +63,7 @@ jobs:
     permissions: {}
     if: failure()
     steps:
-      - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3
+      - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1
```

pinact edits GitHub Workflow files and pins versions of Actions and Reusable Workflows.

## Install

pinact is written in Go. So you only have to install a binary in your `PATH`.

There are some ways to install pinact.

1. Homebrew: `brew install suzuki-shunsuke/pinact/pinact`
1. [aqua](https://aquaproj.github.io/): `aqua g -i suzuki-shunsuke/pinact`
1. Download a pre built binary from GitHub Releases
1. Build yourself with Go: `go install github.com/suzuki-shunsuke/pinact/cmd/pinact`

## GitHub Access token

pinact calls GitHub REST API to get reference and tags.
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
    ()

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

## LICENSE

[MIT](LICENSE)
