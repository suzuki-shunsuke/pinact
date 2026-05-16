# Configuration File

[JSON Schema](json-schema/pinact.json)

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

Please see [here](old_schema.md).
