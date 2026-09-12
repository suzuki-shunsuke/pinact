---
description: Pin actions referencing a branch by converting the branch to a tag with `--branch-to-tag`. Use when an action referencing `main` or `release/*` isn't pinned, to write the regular expression that opts it in, or to learn which tag the branch is converted to.
---

# Pin branches: `--branch-to-tag`

pinact >= v3.10.0, [#1529](https://github.com/suzuki-shunsuke/pinact/issues/1529)

By default, pinact doesn't pin branches such as `main` or `master`.
[This is intentional.](why_pinact_not_pin.md)
If you want to pin specific branches, you can use the `--branch-to-tag` option.

```sh
pinact run --branch-to-tag '<regular expression matching branch name>'
```

The value is evaluated as a regular expression with partial match, just like `--include` / `--exclude`.
Anchor with `^...$` for an exact match - for short branch names like `main` this is recommended to avoid matching `mainline` etc.
Versions that don't match any of the supplied regexps continue to error out as before.

The branch is converted to the **latest stable tag** of the action. Pre-releases are used only when no stable tag exists.

[`--min-age`](update.md) is honored: when set, tags released within the cooldown window are skipped.

`--branch-to-tag` can be specified multiple times.

e.g.

```sh
pinact run --branch-to-tag '^main$' --branch-to-tag '^release/.*$'
```
