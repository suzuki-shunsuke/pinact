---
description: How pinact works with GitHub Actions workflow lockfiles .github/workflows/actions.lock. Use when asked whether pinact is still needed now that lockfiles are generally available, why pinact doesn't pin an action in a workflow, why pinact reports "the lockfile is missing" or "the lockfile couldn't be read", how reusable workflows are pinned alongside a lockfile, or how actions in self repository actions `$/…` are handled.
---

# Workflow lockfiles

[GitHub Actions workflow lockfiles are generally available](https://github.blog/changelog/2026-10-28-github-actions-workflow-lockfiles-are-generally-available/), but they don't support reusable workflows yet.

ref. [workflow dependency lockfiles](https://docs.github.com/en/enterprise-cloud@latest/actions/concepts/security/workflow-dependency-lockfiles)

> It doesn't pin calls to reusable workflows (`jobs.<job_id>.uses`). To pin a reusable workflow, reference it by its full commit SHA.

So you can lock actions with a lockfile and pin reusable workflows with pinact.
If `.github/workflows/actions.lock` exists, pinact leaves actions locked by the lockfile to the lockfile, and pins the rest as usual.
No configuration is needed.

## How pinact handles actions

pinact reads the lockfile and handles actions in workflow steps of `.github/workflows/*.yml` and `.github/workflows/*.yaml` as follows.

| Action | pinact |
| --- | --- |
| Locked by the lockfile for the workflow | Doesn't change it |
| Not locked, in a workflow onboarded to the lockfile | Doesn't change it and fails with exit code 2 |
| In a workflow not onboarded to the lockfile | Pins it as usual |

pinact reads the lockfile next to each workflow file, so it works the same wherever you run pinact, for instance `cd .github/workflows && pinact run ci.yml`.
A workflow is onboarded if the lockfile's `workflows` lists it.
An action is locked if the lockfile lists it for the workflow.
The lockfile records actions as `owner/repo@ref`, where owner and repo are lowercased and a sub-action path is dropped.
For instance, `actions/cache/save@v4` is locked if the lockfile has `actions/cache@v4` for the workflow.

pinact doesn't change locked actions at all: it neither pins them, updates them with `--update`, nor verifies their version comments.
Pinning a locked action by commit SHA would change its reference, so the lockfile would have to be regenerated.
Run `gh actions-lock` to update locked actions.

The following are pinned as usual even if the lockfile exists:

- Reusable workflows (`jobs.<job_id>.uses`). The lockfile doesn't pin them
- Actions in action files such as `action.yaml`, except [self repository actions](#self-repository-actions). Callers of the action files don't necessarily use the lockfile

```diff
 jobs:
   release:
-    uses: suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml@v10.2.0
+    uses: suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml@e897cae161ab26b8c5471a4749e69432bb4be396 # v10.2.0
   build:
     runs-on: ubuntu-latest
     steps:
       - uses: actions/checkout@v6 # Locked by the lockfile
```

## Self repository actions

A workflow can use an action in its own repository as a self repository action `$/<path>`.
`gh actions-lock` rewrites local actions `./<path>` to this form with `--migrate-local-actions`, because the lockfile doesn't follow local actions.

```yaml
# .github/workflows/ci.yml
steps:
  - uses: $/.github/actions/setup
```

The lockfile locks the actions that `.github/actions/setup/action.yaml` uses as dependencies of `ci.yml`.
So pinact handles actions in the action file like actions in `ci.yml`: it doesn't change locked ones, and fails with exit code 2 for ones the lockfile is missing.
pinact follows `$/<path>` in the action files recursively.
An action file used by several onboarded workflows must be locked for all of them.

pinact finds the lockfile in the nearest ancestor directory of the action file, and scans the workflows next to the lockfile for `$/<path>`.
An action file that no onboarded workflow uses as a self repository action, such as one published for other repositories, is pinned as usual.
Local actions `./<path>` are pinned as usual too, because the lockfile doesn't lock them.

## The lockfile is missing an action

GitHub Actions rejects an onboarded workflow that uses an action the lockfile doesn't lock.
pinact reports it in advance:

```
action cannot be auto-fixed: the lockfile is missing actions/setup-go@v6 for .github/workflows/ci.yml. Regenerate it by running gh actions-lock
```

Pinning the action by commit SHA doesn't help, because the lockfile doesn't lock the SHA either.
Run `gh actions-lock` to regenerate the lockfile.

## The lockfile couldn't be read

If pinact fails to read the lockfile, for instance because the lockfile is invalid or its schema version is newer than pinact supports, pinact outputs a warning.
Then pinact can't tell whether actions in workflow steps are locked, so it doesn't change them:

- An action pinned by full commit SHA is left as is
- Any other action makes pinact fail with exit code 2, because it may be neither locked nor pinned

```
action cannot be auto-fixed: the lockfile couldn't be read, so pinact can't tell whether actions/checkout@v6 is locked. Fix the lockfile, or update pinact if the lockfile's schema version is newer
```

Required fields of the lockfile are validated only for entries that the target workflows reference, so a broken entry that no workflow uses doesn't make the lockfile unreadable.
