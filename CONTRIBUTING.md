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

## Documentation

[docs](docs) is embedded in the binary ([docs/doc.go](docs/doc.go)) and served by `pinact docs list` and `pinact docs show <name>`, so the README and the embedded documentation stay the same thing. The command is [cobra-util](https://github.com/suzuki-shunsuke/cobra-util)'s `docs` package; pinact only supplies the files.

Every document requires a YAML frontmatter with a `description` field, and holds nothing else in it. It is all a coding agent sees when it decides whether to open the document, so name the symptoms a reader arrives with rather than the subject alone. [docs/docs_test.go](docs/docs_test.go) fails if a document has no description.

A document in a subdirectory is named by its path, such as `pinact docs show codes/005`. A new subdirectory has to be added to the `go:embed` patterns in [docs/doc.go](docs/doc.go); a new document in an existing directory is served by dropping the file in.

The README links every document from its Documentation section, so add a new document there too. Details belong in a document rather than in the README: the README is what someone reads to decide whether to use pinact, and only the documents are served by `pinact docs`, which is what a coding agent reads. Write each document so it can be read alone, since `pinact docs show` prints one document and nothing around it, and link across documents instead of building on them.

## Add tests

In addition to Go's unit tests, we run integration tests in CI.

- [testdata](testdata)
- [workflow](.github/workflows/workflow_call_integration_test.yaml)

If you change pinact's behaviour, please add tests.
Tests also make how the behaviour is changed clear.
