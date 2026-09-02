---
description: Pin only the lines a diff changed with `--diff-file`. Use to introduce pinact to a repository gradually, to check only the lines a pull request touches, or to read the diff from stdin or from pr-unified-diff-action.
---

# Pin only changed lines

pinact >= v4.0.0

pinact supports pinning only changed lines using the `--diff-file` option.
This is useful to introduce pinact gradually.

```sh
pinact run --diff-file diff.txt
```

diff.txt must be the unified diff format.

[example](../testdata/diff-file/diff.txt)

```diff
diff --git a/testdata/diff-file/action.yaml b/testdata/diff-file/action.yaml
index 0000001..0000002 100644
--- a/testdata/diff-file/action.yaml
+++ b/testdata/diff-file/action.yaml
@@ -7,5 +7,5 @@ jobs:
     permissions: {}
     steps:
       - uses: actions/checkout@v3.6.0
-      - uses: actions/setup-go@v3.5.0
+      - uses: actions/setup-go@v4.0.0
       - uses: actions/cache@v3.3.1
```

`--diff-file -` means reading from stdin:

```sh
cat diff.txt | pinact run --diff-file -
```

You can generate a diff file via GitHub Actions using [pr-unified-diff-action](https://github.com/suzuki-shunsuke/pr-unified-diff-action).

```yaml
- uses: suzuki-shunsuke/pr-unified-diff-action@c932c1df5f577028d8ca05d2d3c0c059072d8821 # v0.0.1
  id: diff
- run: pinact run --diff-file "$DIFF_FILE"
  env:
    DIFF_FILE: ${{ steps.diff.outputs.diff_path }}
```
