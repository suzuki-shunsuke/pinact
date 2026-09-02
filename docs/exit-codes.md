---
description: What pinact's exit codes mean. Use when a run exits 1, 2, or 3 and you need to tell "something needs pinning" from "an action can't be fixed automatically" or from a GitHub API failure.
---

# Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Everything is pinned, or pinact fixed it |
| 1 | `--fix=false` was set and something needs pinning |
| 2 | An action cannot be auto-fixed (branch reference, [missing version comment on a SHA pin](codes/005.md), `--verify-comment` mismatch, or `--min-age` violation) |
| 3 | GitHub API error, invalid CLI flag combination, or other unexpected error |

The exit code 1 is the one CI is meant to fail on: it says the files aren't pinned, and running pinact without `--fix=false` fixes them.

The exit code 2 says pinact can't fix the file itself, so it names what has to be decided by a person: a branch reference ([which pinact doesn't pin by default](why-pinact-not-pin.md), and [`--branch-to-tag`](branch-to-tag.md) opts in), a [SHA pin without a version comment](codes/005.md), a [version comment that doesn't match its SHA](codes/001.md), or a release that is [younger than the minimum release age](update.md).

The exit code 3 is pinact failing rather than the workflow files being wrong, such as a GitHub API error (often [a rate limit, which an access token raises](github-token.md)) or a flag combination that can't be honored.
