# Why doesn't pinact pin some actions?

> [!TIP]
> Since v3.10.0, the [`--branch-to-tag`](../README.md#pin-branches) option lets you opt-in to pinning specific branches to the latest stable tag of an action.

In some cases pinact doesn't pin versions intentionally, which may confuse you.
So we describe the reason here.

By default, pinact doesn't pin actions whose versions aren't semver (e.g. `main`, `master`, `release/v1`).
This is because pinact is designed as a safe tool so that it doesn't change workflows behaviour.
pinact pins actions but doesn't change SHA of actions at the moment when pinact pins versions.

This design enables you to accept changes by pinact safely.

For instance, pinact changes the version `v1` to `v1.1.0` if their SHA are equivalent.
If there are no semver whose SHA is same with `v1`, pinact doesn't change the version.

And pinact doesn't change versions which aren't semver.
For instance, pinact doesn't change the version `main`.

```yaml
uses: actions/checkout@main
```

We don't want to pin `main` to full commit length SHA like the following because we can't update this following semantic versioning.

```yaml
uses: actions/checkout@85e6279cec87321a52edac9c87bce653a07cf6c2 # main
```

Tools like Renovate can update the SHA, but it's not safe at all as `main` branch isn't stable.

And we don't want to change `main` to the latest semver like the following because SHA is changed and workflows may be broken.

```yaml
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # 4.2.2
```

We don't want to pin branches as SHA of branches is changed.

pinact doesn't check if a version is a tag or a branch because we would like to reduce the number of API calls as much as possible.
If a version isn't semver, pinact judges it may be a branch so pinact doesn't pin it.

Please see also [#926](https://github.com/suzuki-shunsuke/pinact/issues/926).
