---
description: Pass a GitHub access token to pinact so its API calls aren't rate limited. Use for `PINACT_GITHUB_TOKEN` and `GITHUB_TOKEN`, for the ghtkn integration and `PINACT_GHTKN`, or for storing a token in the OS keyring with `pinact token set` and `PINACT_KEYRING_ENABLED`.
---

# GitHub access token

pinact calls GitHub REST API to get commit hashes and tags.
You can pass GitHub Access token via environment variable `PINACT_GITHUB_TOKEN` or `GITHUB_TOKEN`.
If no GitHub Access token is passed, pinact calls GitHub REST API without access token.
About GitHub Enterprise Server, see also [GitHub Access Token for GHES](ghes.md).

## Manage GitHub Access token using ghtkn

pinact >= v3.8.0

[You can obtain a GitHub App User Access Token by ghtkn integration](https://github.com/suzuki-shunsuke/ghtkn).
[About ghtkn, please see the document of ghtkn.](https://github.com/suzuki-shunsuke/ghtkn/blob/main/docs/go-sdk.md)
As the document of ghtkn describes, the integration is enabled by default if the configuration file `ghtkn.yaml` exists, but pinact has the specific `PINACT_GHTKN` environment variable to disable it.

## Manage GitHub Access token using Keyring

pinact >= v3.1.0

You can manage a GitHub Access token using secret store such as [Windows Credential Manager](https://support.microsoft.com/en-us/windows/accessing-credential-manager-1b5c916a-6a16-889f-8581-fc16e8165ac0), [macOS Keychain](https://en.wikipedia.org/wiki/Keychain_(software)), and [GNOME Keyring](https://wiki.gnome.org/Projects/GnomeKeyring).

1. Configure a GitHub Access token by `pinact token set` command:

```console
$ pinact token set
Enter a GitHub access token: # Input GitHub Access token
```

or you can also pass a GitHub Access token via standard input:

```sh
echo "<github access token>" | pinact token set --stdin
```

2. Enable the feature by setting the environment variable `PINACT_KEYRING_ENABLED`:

```sh
export PINACT_KEYRING_ENABLED=true
```

Note that if the environment variable `GITHUB_TOKEN` is set, this feature gets disabled.

You can remove a GitHub Access token from keyring by `pinact token rm` command:

```sh
pinact token rm
```
