# Contributing

Please read the following document.

- https://github.com/suzuki-shunsuke/oss-contribution-guide

## How To Develop

We use [aqua](https://aquaproj.github.io/) as a CLI version manager and [cmdx](https://github.com/suzuki-shunsuke/cmdx) as a task runner.

[How to install aqua](https://aquaproj.github.io/docs/install)

```sh
aqua i # Install development tools including cmdx
```

Show tasks:

```sh
cmdx help
```

Test:

```sh
cmdx t
```

Lint:

```sh
cmdx v # go vet
cmdx l # golangci-lint
```

## Add tests

In addition to Go's unit tests, we run integration tests in CI.

- [testdata](testdata)
- [workflow](.github/workflows/workflow_call_integration_test.yaml)

If you change pinact's behaviour, please add tests.
Tests also make how the behaviour is changed clear.
