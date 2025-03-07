# Install

pinact is written in Go. So you only have to install a binary in your `PATH`.

There are some ways to install pinact.

1. [Homebrew](#homebrew)
1. [Scoop](#scoop)
1. [aqua](#aqua)
1. [GitHub Releases](#github-releases)
1. [Build an executable binary from source code yourself using Go](#build-an-executable-binary-from-source-code-yourself-using-go)

## Homebrew

You can install pinact using [Homebrew](https://brew.sh/).

[Homebrew Core Formula: pinact](https://formulae.brew.sh/formula/pinact)

```sh
brew install pinact
```

Or

```sh
brew install suzuki-shunsuke/pinact/pinact
```

## Scoop

You can install pinact using [Scoop](https://scoop.sh/).

```sh
scoop bucket add suzuki-shunsuke https://github.com/suzuki-shunsuke/scoop-bucket
scoop install pinact
```

## aqua

You can install pinact using [aqua](https://aquaproj.github.io/).

```sh
aqua g -i suzuki-shunsuke/pinact
```

## Build an executable binary from source code yourself using Go

```sh
go install github.com/suzuki-shunsuke/pinact/cmd/pinact@latest
```

## GitHub Releases

You can download an asset from [GitHub Releases](https://github.com/suzuki-shunsuke/pinact/releases).
Please unarchive it and install a pre built binary into `$PATH`. 

### Verify downloaded assets from GitHub Releases

You can verify downloaded assets using some tools.

1. [GitHub CLI](https://cli.github.com/)
1. [slsa-verifier](https://github.com/slsa-framework/slsa-verifier)
1. [Cosign](https://github.com/sigstore/cosign)

### 1. GitHub CLI

You can install GitHub CLI by aqua.

```sh
aqua g -i cli/cli
```

```sh
version=v1.0.0
asset=pinact_darwin_arm64.tar.gz
gh release download -R suzuki-shunsuke/pinact "$version" -p "$asset"
gh attestation verify "$asset" \
  -R suzuki-shunsuke/pinact \
  --signer-workflow suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml
```

### 2. slsa-verifier

You can install slsa-verifier by aqua.

```sh
aqua g -i slsa-framework/slsa-verifier
```

```sh
version=v1.0.0
asset=pinact_darwin_arm64.tar.gz
gh release download -R suzuki-shunsuke/pinact "$version" -p "$asset" -p multiple.intoto.jsonl
slsa-verifier verify-artifact "$asset" \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/suzuki-shunsuke/pinact \
  --source-tag "$version"
```

### 3. Cosign

You can install Cosign by aqua.

```sh
aqua g -i sigstore/cosign
```

```sh
version=v1.0.0
checksum_file="pinact_${version#v}_checksums.txt"
asset=pinact_darwin_arm64.tar.gz
gh release download "$version" \
  -R suzuki-shunsuke/pinact \
  -p "$asset" \
  -p "$checksum_file" \
  -p "${checksum_file}.pem" \
  -p "${checksum_file}.sig"
cosign verify-blob \
  --signature "${checksum_file}.sig" \
  --certificate "${checksum_file}.pem" \
  --certificate-identity-regexp 'https://github\.com/suzuki-shunsuke/go-release-workflow/\.github/workflows/release\.yaml@.*' \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "$checksum_file"
cat "$checksum_file" | sha256sum -c --ignore-missing
```
