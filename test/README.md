# Overview

This portion of the boundless-operator tree contains all the tests. 
These are written as Go test files.

## Unit Tests

To run unit test:
```shell
make unit
``` 

To run both unit and integration test:
```shell
make test
```

> Currently, this will run all tests under `/pkg` folder

## Integration Tests

Integration tests are focused on testing functionality of a controller or interaction between two or more controllers. 
The integration tests are based on [envtest](https://github.com/kubernetes-sigs/controller-runtime/tree/main/pkg/envtest).

Currently, all integration tests resides under `/controller` package. 

To run all integration test:
```shell
make integration
```

To run both unit and integration test:
```shell
make test
```

### EnvTest

`EnvTest` is a testing environment that is provided by the [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime
) project. This environment spins up a local instance of `etcd` and the `kube-apiserver`. This allows tests to be 
executed in an environment very similar to a real environment.

See the KubeBuilder article on [KubeBuilder - Writing controller tests](https://kubebuilder.io/cronjob-tutorial/writing-tests) to get an overview
of how `envtest` based tests works.

The integration tests also use [ginkgo](https://onsi.github.io/ginkgo/) and [gomega](https://onsi.github.io/gomega/). 
Its is helpful to get familiarize with these libraries.

> Note: Since we are running only one _instance_ of `envtest`, the state is shared among all tests. This means, we must
clean up the resources our tests creates and provide a clean slate for other tests.

#### Gotchas with `envtest`

`EnvTest` does not entirely replicate a Kubernetes environment. Important things to remember:
* `EnvTest` does not support namespace deletion. Deleting a namespace will seem to succeed, but the namespace will just be put in a Terminating state, and never actually be reclaimed.
* Because there are no controllers monitoring built-in resources, the built-in objects do not get deleted

More details here: https://book.kubebuilder.io/reference/envtest.html#testing-considerations

### Further Reading:
* [KubeBuilder - Writing controller tests](https://kubebuilder.io/cronjob-tutorial/writing-tests )
* [KubeBuilder - Configuring envtest for integration tests](https://book.kubebuilder.io/reference/envtest.html)
  * [Testing considerations](https://book.kubebuilder.io/reference/envtest.html#testing-considerations)
* Testing instructions for Cluster API project: https://cluster-api.sigs.k8s.io/developer/testing.html: 
* [ginkgo](https://onsi.github.io/ginkgo/) 
* [gomega](https://onsi.github.io/gomega/).

## E2E Tests

The E2E tests allow us to run a real deployment of Boundless Operator
and test the entire system. It ensures the system performs all its 
intended functions and meets the user's requirements.

The e2e tests reside under [test/e2e/](e2e) directory.

### Running e2e tests
To run all the tests, run `make e2e` command. 

> By default, the e2e tests will run against the latest released version of the operator. To run tests with a specific version of the operator, see the usage of `E2E_TEST_FLAGS` environment variable.

Use the `E2E_TEST_FLAGS` to pass flags to the test binary. For example:
```shell
# Run tests with verbose output
E2E_TEST_FLAGS="-test.v" make e2e

# To run specific test
E2E_TEST_FLAGS="-test.run ^TestOperatorInstall" make e2e

# To run tests with a specific version of the operator.
# Ensure that the image already exists in the registry.
E2E_TEST_FLAGS="-img mirantiscontainers/boundless-operator:dev" make e2e
```

### Running e2e in CI
The workflow for e2e tests is automatically initiated when a PR is created and merged. 
The e2e tests are run against the PR image build.

See: [.github/workflows/e2e.yml](..%2F.github%2Fworkflows%2Fe2e.yml)
