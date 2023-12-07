# Overview

This portion of the boundless-operator tree contains all the tests. 
These are written as Go test files.


# E2E Tests
The E2E tests allow us to run a real deployment of Boundless-Operator
and test the entire software system. It ensures the system performs all its 
intended functions and meets the user's requirements.


## Running e2e tests

The e2e tests reside under boundless-operator/test/e2e directory and are organized 
under different folders.

To run all the tests, go to the root directory and run `make e2e` command . 

If you want to run a specific test, for example `bopinstall`, you can use the following command.

`go test -v ./test/e2e/bopinstall`


### Running e2e in CI
The workflow for e2e tests is automatically initiated whenever a PR is created.

