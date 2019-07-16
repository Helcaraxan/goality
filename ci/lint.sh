#!/usr/bin/env bash
# vim: set tabstop=4 shiftwidth=4 noexpandtab
set -u -e -o pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
	echo "This script is only intended to be run on Linux as the used CLI tools might not be available or differ in their semantics."
	exit 1
fi

# Check linter versions are specified.
if [[ -z ${GOLANGCI_VERSION-} ]]; then
	echo "Please specify the 'golangci-lint' version that should be used via the 'GOLANGCI_VERSION' environment variable."
	exit 1
elif [[ -z ${SHELLCHECK_VERSION} ]]; then
	echo "Please specify the 'shellcheck' version that should be used via the 'SHELLCHECK_VERSION' environment variable."
	exit 1
fi

# Retrieve linters.
## golangci-lint
if [[ -z "$(command -v golangci-lint)" ]] || ! grep "${GOLANGCI_VERSION}" <<<"$(golangci-lint --version)"; then
	echo "Downloading golangci-lint@${GOLANGCI_VERSION}."
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | BINARY="golang-ci" bash -s -- -b "${GOPATH}/bin" "v${GOLANGCI_VERSION}"
else
	echo "Found installed golangci-lint@${GOLANGCI_VERSION}."
fi

## shellcheck
if [[ -z "$(command -v shellcheck)" ]] || grep "${SHELLCHECK_VERSION}" <<<"$(shellcheck --version)"; then
	echo "Downloading shellcheck@${SHELLCHECK_VERSION}."
	curl -sfL "https://storage.googleapis.com/shellcheck/shellcheck-v${SHELLCHECK_VERSION}.linux.x86_64.tar.xz" | tar -xJv
	PATH="${PWD}/shellcheck-v${SHELLCHECK_VERSION}:${PATH}"
else
	echo "Founc installed shellcheck@${SHELLCHECK_VERSION}."
fi

# shfmt
go install mvdan.cc/sh/cmd/shfmt
PATH="${GOPATH}/bin:${PATH}"

# Run linters.
golangci-lint run ./...

shell_failure=0
shell_vim_directives="# vim: set tabstop=4 shiftwidth=4 noexpandtab"
while read -r shell_file; do
	echo "Linting ${shell_file}"

	pushd "$(dirname "${shell_file}")"
	shell_file="$(basename "${shell_file}")"
	shellcheck --check-sourced --external-sources --shell=bash --severity=style "${shell_file}" || shell_failure=1
	if ! grep -q "^${shell_vim_directives}$" "${shell_file}"; then
		echo "'${shell_file}' is missing the compulsory VIm directives: ${shell_vim_directives}"
		shell_failure=1
	fi
	popd
done <<<"$(shfmt -f .)"
if ((shell_failure == 1)); then
	echo "Errors were detected while linting shell scripts."
	exit 1
fi

shfmt -s -d .

# Check that dependencies are correctly being maintained.
go mod tidy
git diff --exit-code --quiet || (
	echo "Please run 'go mod tidy' to clean up the 'go.mod' and 'go.sum' files."
	false
)

# Check that generated code is up-to-date.
go generate ./...