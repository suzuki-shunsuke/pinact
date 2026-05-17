# Configuration File

[JSON Schema](../json-schema/pinact.json)

A configuration file is optional.
pinact supports a configuration file `.pinact.yaml`, `.github/pinact.yaml`, `.pinact.yml` or `.github/pinact.yml`.
You can also specify the configuration file path by the environment variable `PINACT_CONFIG` or command line option `-c`.
You can generate a configuration file by `pinact init`.

```sh
pinact init [<configuration file path>]
```

As of pinact v2.2.0, pinact configuration file has a schema version.

```yaml
version: 3
```

In general, you should use the latest schema version.

## Migration: `pinact migrate`

`pinact migrate` command migrates your configuration file to the latest schema version.

```sh
pinact migrate
```

## JSON Schema

- [pinact.json](../json-schema/pinact.json)
- https://raw.githubusercontent.com/suzuki-shunsuke/pinact/refs/heads/main/json-schema/pinact.json

If you look for a CLI tool to validate configuration with JSON Schema, [ajv-cli](https://ajv.js.org/packages/ajv-cli.html) is useful.

```sh
ajv --spec=draft2020 -s json-schema/pinact.json -d pinact.yaml
```

### Input Complementation by YAML Language Server

[Please see the comment too.](https://github.com/szksh-lab/.github/issues/67#issuecomment-2564960491)

Version: `main`

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/suzuki-shunsuke/pinact/main/json-schema/pinact.json
```

Or pinning version:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/suzuki-shunsuke/pinact/v1.1.2/json-schema/pinact.json
```

## Global Configuration File

pinact supports a global configuration file for user-wide defaults.

Global Configuration File Paths:

1. Linux, macOS:
    1. `$XDG_CONFIG_HOME/pinact/pinact.yaml` if `$XDG_CONFIG_HOME` is set
    2. `~/.config/pinact/pinact.yaml`
2. Windows:
    1. `%APPDATA%\pinact\pinact.yaml`

A global configuration file is ignored if a local configuration file is found.
`pinact init -g` creates a global configuration file if it doesn't exist.

```sh
pinact init -g
```

## Schema v3 (latest)

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

# Default min-age in days (optional)
# pinact >= v4.0.0
min_age:
  value: 7
  always: true

rules:
  - ignore: true
    conditions:
      - expr: |
          ActionName == "slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml" && ActionVersion matches "v\\d+\\.\\d+\\.\\d+"
      - expr: |
          ActionName matches "suzuki-shunsuke/.*" && ActionVersion == "main"
  - min_age: 0
    conditions:
      - expr: |
          ActionName matches "suzuki-shunsuke/.*" && ActionVersion == "main"
```

### `files`

This is optional.
A list of target files.

### `files[].pattern`

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

### `rules`

pinact >= v4.0.0

This is optional. A list of rules that override per-action settings for actions matching expression conditions.

Each rule has match conditions and one or more override fields (`ignore`, `min_age`). When pinact processes an action, every rule's conditions are evaluated in declaration order. For each rule that matches, its override fields are merged on top of previous matches: later rules override earlier ones, but only for the fields they explicitly set.

For new configurations, prefer `rules` with `ignore: true` over `ignore_actions` for more flexibility (`rules` can match on owner, repo, version comment, etc.).

e.g.

```yaml
rules:
  # Skip actions whose ref is a branch name in our own repos.
  - ignore: true
    conditions:
      - expr: |
          ActionRepoOwner == "suzuki-shunsuke" && ActionVersion == "main"
  # Lower the min-age threshold for trusted actions.
  - min_age: 0
    conditions:
      - expr: |
          ActionRepoFullName == "actions/checkout"
```

#### `rules[].ignore`

This is optional. If `true`, pinact skips pin/update/error reporting for the matched action.

#### `rules[].min_age`

This is optional. Overrides the min-age threshold (in days) for the matched action. Setting it to `0` disables the min-age check for the action.

The effective min-age for an action is resolved in this order: CLI flag `-min-age` > matching rules > top-level `min_age`.

#### `rules[].conditions`

This is required. A list of match conditions. The rule matches if **any** of its conditions evaluates to `true` (OR semantics). Each rule must have at least one condition.

#### `rules[].conditions[].expr`

This is required. A boolean expression evaluated against the action being processed. The [expr language](https://expr-lang.org/docs/language-definition) is used.

The following variables are available:

| Variable | Description | Example |
|---|---|---|
| `ActionName` | Full action name | `slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml` |
| `ActionRepoOwner` | Repository owner | `slsa-framework` |
| `ActionRepoName` | Repository name | `slsa-github-generator` |
| `ActionRepoFullName` | `<owner>/<repo>` | `slsa-framework/slsa-github-generator` |
| `ActionVersion` | The action's ref (commit SHA, tag, or branch) | `v1.10.0`, `main`, `68bad40...` |
| `VersionComment` | Existing `# <tag>` comment on the line, if any | `v1.10.0` |

Expressions are validated and compiled at startup. Syntax errors, references to undefined variables, and non-boolean expressions are surfaced as configuration errors.

### `min_age`

pinact >= v4.0.0

This is optional. The default min-age in days for the min-age check. When set, pinact checks that every action's pinned commit is at least this many days old, and exits with code 2 on violation.

The top-level value can be overridden by the CLI flag `-min-age` (highest precedence) or per-action by `rules[].min_age`.

### `ignore_actions`

This is optional. A list of ignored actions and reusable workflows.

> [!NOTE]
> For new configurations, consider using [`rules`](#rules) with `ignore: true` instead. `rules` can match on the repository owner, full name, version comment, and arbitrary expressions, while `ignore_actions` only matches on name and ref regexps.

### `ignore_actions[].name`

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

### `ignore_actions[].ref`

This is required.
A regular expression of ignored action versions (branch, tag, or commit hash).

> [!WARNING]
> Regular expressions must match with action versions exactly.
> For instance, `ref: main` doesn't match with `malicious-main`

### `ghes`

[See GitHub Enterprise Support](ghes.md).

### `separator`

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

## Old Schemas

Please see [here](old_schema.md).
