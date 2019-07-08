# Goality - Quality for your Go codebase

`goality` is a tool that helps Go project maintainers get a picture of the health and quality of
their codebase. It relies on linters and meta-linters such as [`golangci-lint`] to analyse the
project and uses the results to compile a report of the findings. The contents and level-of-detail
of the produced reports is configurable.

## Table of Contents

- [Detailed features](#detailed-features)
  - [Commands](#commands)
    - [`goality run`](#goality-run)
- [Example output](#example-output)
  - [Lint issue prevalence](#lint-issue-prevalence)

## Detailed features

### Commands

#### `goality run`

Runs an analysis on the given path and produces a high-level issue prevalence report. The linters
that will be run, their configuration as well as the granularity of the report can be configured.

## Example output

### Lint issue prevalence
