# History

1. 2025-12 `--cooldown` was renamed to `--min-age`

https://github.com/suzuki-shunsuke/pinact/issues/1265#issuecomment-3632228752

## Background

The `--cooldown` option was originally implemented in PR #1266.
However, the option name was renamed to `--min-age` with alias `-m` due to the following reasons:

1. **Alias conflict**: The short alias `-c` was already used by `--config` option, so `--cooldown` could not have a convenient short alias.
2. **Desire for short alias**: Since `cooldown` is a relatively long option name, a short alias was desired for usability.
3. **Semantic clarity**: `--min-age` better describes the option's behavior - it specifies the minimum age (in days) that a version must have to be considered for updates.

### Changes Made

- Renamed `--cooldown` to `--min-age`
- Added `-m` as a short alias
- Updated all internal variable names from `Cooldown` to `MinAge`
- Updated documentation and error messages
