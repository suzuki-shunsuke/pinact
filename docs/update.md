---
description: Update actions to their latest versions and hold back releases that are too new. Use for `--update`, for the minimum release age (cooldown) via `--min-age`, `--verify-min-age`, `PINACT_MIN_AGE`, `min_age`, or when an update fails because no release meets the minimum age.
---

# Updating actions

## Update actions: `--update`

Update actions to latest versions:

```sh
pinact run --update
```

## Minimum release age (cooldown): `--min-age`, `--verify-min-age`

pinact supports two kinds of minimum release age checks:

1. Verify current versions: Verify if current action versions meet the minimum release age requirement
1. Verify new versions: Exclude versions that don't meet the minimum release age requirement when updating actions (`--update`)
    1. If no release meeting the given minimum age is found, pinact will exit with an error.

This helps reduce supply chain security risks.

By default, no minimum release age is set.
You can set the minimum release age by some methods:

1. `--min-age <minimum release age>`: Set the minimum release age in days

```sh
pinact run --min-age 7
```

2. Environment variable `PINACT_MIN_AGE`
3. Configuration file `.pinact.yml`
    1. `.rules[].min_age`: A rule specific minimum release age in days
    1. `.min_age.value`: The default minimum release age in days

```yaml
min_age:
  value: 7
rules:
  - min_age: 0
    conditions:
      - expr: |
          ActionRepoOwner == "suzuki-shunsuke"
```

It may be wasteful to verify all current versions against the minimum release age every time pinact runs.
Therefore, current versions are verified using the min_age setting in .pinact.yml and `PINACT_MIN_AGE` only when --verify-min-age is set or .min_age.always is true.

```sh
pinact run --verify-min-age
```

Or

```yaml
min_age:
  value: 7
  always: true # default is false
```

On the other hand, when updating actions min_age setting is always used to filter new versions.

- For GitHub Releases, the `PublishedAt` date is checked
- For tags, the commit's `Committer.Date` is checked (requires additional API call)

A violation of the minimum release age is not something pinact can fix, so it [exits with the code 2](exit-codes.md).
