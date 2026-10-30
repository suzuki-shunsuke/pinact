---
description: How pinact relates to GitHub Actions workflow lockfiles, and how to pin only reusable workflows. Use when asked whether pinact is still needed now that lockfiles are generally available, when actions are locked by a lockfile but `jobs.<job_id>.uses` must still be pinned to a full commit SHA, or to make pinact skip every action except reusable workflows.
---

# Pin only reusable workflows alongside workflow lockfiles

[GitHub Actions workflow lockfiles are generally available](https://github.blog/changelog/2026-10-28-github-actions-workflow-lockfiles-are-generally-available/), but they don't support reusable workflows yet.

ref. [workflow dependency lockfiles](https://docs.github.com/en/enterprise-cloud@latest/actions/concepts/security/workflow-dependency-lockfiles)

> It doesn't pin calls to reusable workflows (`jobs.<job_id>.uses`). To pin a reusable workflow, reference it by its full commit SHA.

So you can lock actions with a lockfile and pin only reusable workflows with pinact.
pinact also updates them with `--update` and checks them with `--check`.

pinact has no dedicated option for this, but a [rule](config.md#rules) can ignore everything that isn't a reusable workflow.

## Configuration

The name of a reusable workflow always contains `/.github/workflows/`, such as `suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml`.
Ignore every action whose name doesn't contain it.

```yaml
version: 3
rules:
  - ignore: true
    conditions:
      - expr: "!(ActionName contains '/.github/workflows/')"
```

With this configuration, pinact pins only reusable workflows:

```diff
 jobs:
   release:
-    uses: suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml@v10.2.0
+    uses: suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml@e897cae161ab26b8c5471a4749e69432bb4be396 # v10.2.0
   build:
     runs-on: ubuntu-latest
     steps:
       - uses: actions/checkout@v4 # Left to the lockfile
```

An ignored action is skipped entirely: pinact neither pins it, updates it with `--update`, nor reports it with `--check`.

## Per run

To do this for a single run without changing the configuration file, use [`--include`](include_exclude.md):

```sh
pinact run -i '/\.github/workflows/'
```
