# GitHub Enterprise Server (GHES) Support

v3.6.0 [#839](https://github.com/suzuki-shunsuke/pinact/issues/839) [#1275](https://github.com/suzuki-shunsuke/pinact/pull/1275)

pinact also supports pinning versions of GitHub Actions hosted on GitHub Enterprise Server (GHES).
If the GHES support is enabled, pinact searches actions in GHES.

## Fallback to github.com

The fallback to github.com is disabled by default.
All actions are searched on the GHES instance only.
If the fallback is enabled, repositories of actions are first searched on the GHES instance. If repositoires are not found (404), pinact falls back to github.com. This is suitable when [GitHub Connect is enabled](https://docs.github.com/en/enterprise-server@3.19/admin/managing-github-actions-for-your-enterprise/managing-access-to-actions-from-githubcom/enabling-automatic-access-to-githubcom-actions-using-github-connect).

## GitHub Access Token for GHES

Set a GitHub Access Token for GHES using one of the following environment variables (checked in order):

1. `PINACT_GHES_TOKEN`
2. `GHES_TOKEN`
3. `GITHUB_TOKEN_ENTERPRISE`
4. `GITHUB_ENTERPRISE_TOKEN`

```sh
export GHES_TOKEN=xxx
```

`GITHUB_TOKEN` is used for github.com.

## Configuration File For GHES

GHES configuration is required via configuration file or environment variables.
The configuration file takes precedence over the environment variables.

```yaml
ghes:
  api_url: https://ghes.example.com
  fallback: true # optional, default is false
```

- `api_url`: API URL of the GHES instance. Can also be set via environment variables.
- `fallback`: Whether to fallback to github.com when a repository is not found on GHES. Default is `false`.

## Environment Variables For GHES

You can also configure GHES using environment variables instead of a configuration file.

- `GHES_API_URL`
- `PINACT_GHES_FALLBACK`

```sh
export GHES_API_URL=https://ghes.example.com
export PINACT_GHES_FALLBACK=true
```

If `GHES_API_URL` is not set, `GITHUB_API_URL` will be used instead.
This is convenient when running on GitHub Actions hosted on GHES.

## Conditions for Enabling GHES

GHES mode is enabled when any of the following conditions are met:

1. `ghes.api_url` is configured in the configuration file
2. `GHES_API_URL` environment variable is set
3. `GITHUB_API_URL` environment variable is set and is not `https://api.github.com`
