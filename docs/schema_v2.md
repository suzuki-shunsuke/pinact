# Schema v2

```yaml
version: 2
```

`pinact <= v2.2.0`

pinact v2.2.0 or older supports this schema version.

## `files`

This is optional.
A list of target files.

## `files[].pattern`

This is required.
A regular expression of target files.
If files are passed via positional command line arguments, the configuration is ignored.

e.g.

```yaml
files:
  - pattern: ^\.github/workflows/.*\\.ya?ml$
```

> [!WARNING]
> Regular expressions doesn't necessarily match with action names exactly.
> For instance, `pattern: action\\.yaml` matches with `foo/action\\.yaml`

## `ignore_actions`

This is optional. A list of ignored actions and reusable workflows.

## `ignore_actions[].name`

This is required.
A regular expression of ignored actions and reusable workflows.

```yaml
ignored_actions:
  - name: actions/.*
    ref: main
```

> [!WARNING]
> Regular expressions doesn't necessarily match with action names exactly.
> For instance, `name: actions/` matches with `actions/checkout`

## `ignore_actions[].ref`

`pinact >= v2.1.0`

This is required.
A regular expression of ignored action versions (branch, tag, or commit hash).

> [!WARNING]
> Regular expressions must match with action names exactly.
> For instance, `ref: main` matches with `main`
