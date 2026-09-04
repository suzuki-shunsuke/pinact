---
description: Output the result in the SARIF format for reviewdog and GitHub Code Scanning. Use for `--format sarif`, to post review comments on a pull request with reviewdog, or to upload the findings to GitHub code scanning.
---

# SARIF

pinact >= v3.7.0 [#1294](https://github.com/suzuki-shunsuke/pinact/pull/1294)

pinact can output the result in [the SARIF format](https://sarifweb.azurewebsites.net/).

```sh
pinact run --format sarif
```

This format is useful to integration tools like [reviewdog](https://github.com/reviewdog/reviewdog) and [GitHub SARIF Code Scanning](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning).

`--format sarif` implies `--fix=false`, so files are not modified.
If you want to fix files, use `--fix`.

```sh
pinact run --format sarif --fix
```

## Reviewdog

```sh
pinact run --format sarif |
  reviewdog -f sarif -name pinact -reporter github-pr-review
```

## GitHub SARIF Code Scanning

```yaml
- run: pinact run --format sarif > sarif.json || true
- name: Upload SARIF file
  uses: github/codeql-action/upload-sarif@5d4e8d1aca955e8d8589aabd499c5cae939e33c7 # v4.31.9
  with:
    sarif_file: sarif.json
    category: pinact
```
