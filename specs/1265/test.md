# Test Plan: `--min-age` option

## Validation Tests

### 1. Error when `--min-age` is specified without `-u`

```sh
pinact run --min-age 7
```

**Expected**: Error message indicating `--min-age requires --update (-u) flag`

### 2. Error when `--min-age` is negative

```sh
pinact run -u --min-age -1
```

**Expected**: Error message indicating `--min-age must be a non-negative integer`

### 3. `--min-age 0` should work (no filtering)

```sh
pinact run -u --min-age 0
```

**Expected**: Behaves same as `pinact run -u` (all versions eligible)

## Functional Tests

### 4. Skip recently released versions (releases)

Test with a repository that has GitHub Releases.

```sh
# Use a large min-age to skip recent releases
pinact run -u --min-age 9999 --log-level info
# or using short alias
pinact run -u -m 9999 --log-level info
```

**Expected**:
- Info log messages like `skip release due to cooldown` with tag and published_at
- Action not updated to the latest version

### 5. Skip recently released versions (tags)

Test with a repository that only uses tags (no releases).

```sh
pinact run -u --min-age 9999 --log-level info
```

**Expected**:
- Info log messages like `skip tag due to cooldown` with tag and committed_at
- Action not updated to the latest version

### 6. Update to eligible version

```sh
# Use small min-age that allows some versions
pinact run -u --min-age 30 --log-level info
```

**Expected**:
- Recent versions skipped (info logs)
- Action updated to the latest eligible version (older than 30 days)

### 7. No update when all versions are within min-age

```sh
# Test with a very new action or large min-age
pinact run -u --min-age 9999
```

**Expected**: No changes to the file (current version retained)

## Edge Cases

### 8. Mixed releases and tags

Test with a repository that has both releases and tags with different dates.

**Expected**: Min-age filtering applied consistently

### 9. Commit fetch failure for tags

Test scenario where commit cannot be fetched (e.g., deleted commit, API error).

```sh
pinact run -u --min-age 7 --log-level warn
```

**Expected**:
- Warning log: `skip tag: failed to get commit for cooldown check`
- That tag is skipped

### 10. Stable version filtering combined with min-age

Test with a prerelease current version and stable version:

```yaml
- uses: owner/repo@sha # v1.0.0-beta
```

```sh
pinact run -u --min-age 30
```

**Expected**: Both prerelease filtering and min-age filtering applied

## Test Workflow Files

### Sample workflow for testing

`.github/workflows/test.yaml`:

```yaml
name: Test
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      # Test with actions/checkout (has releases)
      - uses: actions/checkout@v4

      # Test with an action that only has tags
      - uses: suzuki-shunsuke/tfcmt@v1
```

Run:

```sh
pinact run -u --min-age 7 .github/workflows/test.yaml --log-level info
# or
pinact run -u -m 7 .github/workflows/test.yaml --log-level info
```
