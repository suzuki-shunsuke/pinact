---
description: Pin only some of the actions in a file with `--include` and `--exclude`. Use to fix or skip actions matching a regular expression, or to introduce pinact to a repository one owner at a time; `rules` and `ignore_actions` do the same from the configuration file.
---

# Include and exclude specific actions

[#1082](https://github.com/suzuki-shunsuke/pinact/pull/1082) pinact >= v3.4.0

You can fix only specific actions using the `--include (-i) <regular expression>` option.
You can also exclude only specific actions using the `--exclude (-e) <regular expression>` option.

e.g.

```sh
pinact run -i "actions/.*" -i "^aquaproj/aqua-installer$"
```

```sh
pinact run -e "actions/.*" -e "^aquaproj/aqua-installer$"
```

To exclude actions permanently rather than per run, use [`rules` with `ignore: true` in the configuration file](config.md).
