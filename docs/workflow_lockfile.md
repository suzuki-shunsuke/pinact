---
description: How pinact works with GitHub Actions workflow lockfiles .github/workflows/actions.lock. Use when asked whether pinact is still needed now that lockfiles are generally available, why pinact doesn't pin an action in a workflow or an action file, why pinact warns "the workflow isn't in the lockfile" or "the lockfile doesn't record", why pinact fails to read the lockfile, how reusable workflows are pinned alongside a lockfile, or how self repository actions `$/…` are handled.
---

# Workflow lockfiles

[GitHub Actions workflow lockfiles are generally available](https://github.blog/changelog/2026-10-28-github-actions-workflow-lockfiles-are-generally-available/), but they don't support reusable workflows yet.

ref. [workflow dependency lockfiles](https://docs.github.com/en/enterprise-cloud@latest/actions/concepts/security/workflow-dependency-lockfiles)

> It doesn't pin calls to reusable workflows (`jobs.<job_id>.uses`). To pin a reusable workflow, reference it by its full commit SHA.

So you can lock actions with a lockfile and pin reusable workflows with pinact.
If `.github/workflows/actions.lock` exists, pinact leaves the actions of workflows onboarded to the lockfile to the lockfile, and pins the rest as usual.
No configuration is needed.

ref. [#1828](https://github.com/suzuki-shunsuke/pinact/issues/1828)

## How pinact handles actions

A workflow is onboarded if the lockfile's `workflows` lists it.

| Target | pinact |
| --- | --- |
| Steps of an onboarded workflow | Doesn't change them |
| Steps of a workflow that isn't onboarded | Pins them as usual and outputs a warning |
| Steps of an action file used as a [self repository action](#self-repository-actions) `$/<path>` by an onboarded workflow | Doesn't change them |
| Steps of other action files | Pins them as usual |
| Reusable workflows (`jobs.<job_id>.uses`) | Pins them as usual. The lockfile doesn't pin them |

pinact doesn't change actions left to the lockfile at all: it neither pins them, updates them with `--update`, nor verifies their version comments.
Pinning them by commit SHA would change their references, so the lockfile would have to be regenerated.
Run `gh actions-lock` to update them.

```diff
 jobs:
   release:
-    uses: suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml@v10.2.0
+    uses: suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml@e897cae161ab26b8c5471a4749e69432bb4be396 # v10.2.0
   build:
     runs-on: ubuntu-latest
     steps:
       - uses: actions/checkout@v6 # Left to the lockfile
```

pinact reads the lockfile next to each workflow file, so it works the same wherever you run pinact, for instance `cd .github/workflows && pinact run ci.yml`.

## Warnings

Usually all workflows should be onboarded, and the lockfile should record all actions of onboarded workflows.
pinact outputs a warning in the following cases, because it's likely that `gh actions-lock` wasn't run.
They are warnings, not errors.

A workflow isn't onboarded:

```
the workflow isn't in the lockfile. Run gh actions-lock to add it workflow=.github/workflows/release.yml
```

The lockfile doesn't record an action of an onboarded workflow, so GitHub Actions rejects the workflow:

```
the lockfile doesn't record actions/setup-go@v6 for .github/workflows/ci.yml, so GitHub Actions rejects the workflow. Run gh actions-lock to regenerate the lockfile
```

An action is recorded for a workflow if the lockfile lists it in both `workflows.<workflow path>` and `dependencies`.
The lockfile records actions as `owner/repo@ref`, where owner and repo are lowercased and a sub-action path is dropped.
For instance, `actions/cache/save@v4` is recorded if the lockfile has `actions/cache@v4` for the workflow.

## Self repository actions

A workflow can use an action in its own repository as a self repository action `$/<path>`.
`gh actions-lock` rewrites local actions `./<path>` to this form with `--migrate-local-actions`, because the lockfile doesn't follow local actions.

```yaml
# .github/workflows/ci.yml
steps:
  - uses: $/.github/actions/setup
```

The lockfile locks the actions that `.github/actions/setup/action.yaml` uses as dependencies of `ci.yml`.
So pinact handles actions in the action file like actions in `ci.yml`: it doesn't change them, and outputs a warning for ones the lockfile doesn't record.
pinact follows `$/<path>` in the action files recursively.
`$/<path>` at the job level (a reusable workflow) isn't followed.

If an action file is used by both an onboarded workflow and a workflow that isn't onboarded, pinact doesn't change it, because pinning it would break the lockfile of the onboarded workflow.
pinact outputs the warning for the workflow that isn't onboarded.

pinact finds the lockfile in the nearest ancestor directory of the action file, and scans the workflows next to the lockfile for `$/<path>`.
Other action files are pinned as usual, because the lockfile doesn't lock them:

- Action files used as local actions `./<path>`. The lockfile doesn't follow them
- Action files published for other repositories. Callers don't necessarily use the lockfile
- Action files that no workflow uses

To exclude such action files, for instance when their callers are assumed to use the lockfile, list the target files with [`files`](config.md).
[#1829](https://github.com/suzuki-shunsuke/pinact/issues/1829) proposes a way to exclude them with `rules`.

## The lockfile can't be read

If pinact fails to read the lockfile, for instance because the lockfile is invalid or its schema version is newer than pinact supports, pinact fails with exit code 3 without processing any file, because it can't tell which workflows are onboarded.

```
ERR pinact failed error="parse the lockfile: line 1, column 10: lockfile version v0.0.4 is newer than this binary supports (v0.0.3); upgrade the tool that reads this lockfile to a build that supports v0.0.4"
```

Fix the lockfile, or update pinact if the lockfile's schema version is newer.

pinact reads the lockfile only if a target file is a workflow file next to it or an action file under it.
Required fields of the lockfile are validated only for entries that the concerned workflows reference, so a broken entry that no concerned workflow uses doesn't make the lockfile unreadable.
