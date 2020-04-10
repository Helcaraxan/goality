#!/usr/bin/env bash
# vim: set tabstop=4 shiftwidth=4 noexpandtab
set -e -u -o pipefail

if [[ "$(uname -s)" != "Linux" && "$(uname -s)" != "Darwin" ]]; then
	echo "This script is only intended to be run on *nix systems as the used CLI tools might not be available or differ in their semantics."
	exit 1
fi

# Ensure 'golangci-lint' is available with the expected version on the PATH.
GOLANGCI_VERSION="${GOLANGCI_VERSION:="1.24.0"}"
if [[ -z ${GOLANGCI_VERSION} ]]; then
	echo "Please specify the 'golangci-lint' version that should be used via the 'GOLANGCI_VERSION' environment variable."
	exit 1
fi

if [[ -z "$(command -v golangci-lint)" ]] || ! grep "${GOLANGCI_VERSION}" <<<"$(golangci-lint --version)"; then
	echo "Downloading golangci-lint@${GOLANGCI_VERSION}."
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | BINARY="golang-ci" bash -s -- -b "${GOPATH}/bin" "v${GOLANGCI_VERSION}"
else
	echo "Found installed golangci-lint@${GOLANGCI_VERSION}."
fi

# Run all the Go tests with the race detector and generate coverage.
printf "\nRunning Go test...\n"
go test -v -race -coverprofile c.out ./...

# Ensure the binary builds on all platforms.
echo "Testing Linux build."
GOOS="linux" GOARCH="amd64" go build -o goality_linux .
echo "Testing Darwin build."
GOOS="darwin" GOARCH="amd64" go build -o goality_darwin .
echo "Testing Windows build."
GOOS="windows" GOARCH="amd64" go build -o goality_windows.exe .
