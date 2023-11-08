# CI

This document describes the CI/CD pipeline for the project. This document is for those developing on the project to understand everything that is running when they make a change to the code. If you are interested in modifying the CI/CD pipeline, see [README](.github/workflows/README.md) in the workflows directory.

## PRs

CI for a PR will trigger whenever a PR is opened, reopened, or pushed. The jobs ran on a PR are meant to be lightweight enough that we can repeatedly run them (each push) but cover enough of the code that we don't have to create a followup PR to fix things that we've missed. The image will not be pushed to the registry after a successful PR run. The jobs ran on a PR are:

- 'vet' - Check the code changes and make sure they adhere to the standard Golang style guide
- 'test' - Run the unit tests on the code changes
- 'build' - Build an image containing the code changes

## Merging to main

Merging to main runs many of the same tests as a PR to verify that merging the code didn't introduce any new issues. Merging will also run integration tests to verify that the code works with the rest of the system as these tests require more setup and take longer to run. After all of the steps successfully run, the same image will be pushed with the commit `sha` and `dev` tags.

## Releases

A release is triggered when a pre-release is created in the github repo. This will run EVERYTHING from scratch. Starting from zero may take more time but this ensures that nothing slips by us before sending out the release. This includes any static code analysis, unit tests, integration tests, and building the binaries. If everything passes, the same image will pushed with `latest`, `sha`, `semver`, and `dev` tags. This process is documented in [Creating a release](docs/creating-a-release.md).
