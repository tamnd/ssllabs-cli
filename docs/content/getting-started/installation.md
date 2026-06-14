---
title: "Installation"
description: "Install ssllabs from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/ssllabs-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `ssllabs` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/ssllabs-cli/cmd/ssllabs@latest
```

That puts `ssllabs` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/ssllabs-cli
cd ssllabs-cli
make build        # produces ./bin/ssllabs
./bin/ssllabs version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/ssllabs:latest --help
```

## Checking the install

```bash
ssllabs version
```

prints the version and exits.
