## Contributing to Blueprint Operator
We welcome contributions to Blueprint Operator in the form of Pull Requests or submitting Issues. If you encounter a problem with Blueprint Operator or want to suggest an improvement, please submit an Issue. If you find a bug, please tell us about it by submitting an Issue or Pull Request. Please make sure you are testing against the latest version of Blueprint Operator when you are submitting a bug. Provide as much detail as you can.

## Developer Guide
### Prerequisites
* Read and follow our [Code of Conduct](./CODE-OF-CONDUCT.md).

### Developer Setup
Where needed, each piece of required software will have separate instructions for Linux and MacOS.

#### Setting up Linux
On Linux, most development tools are pre-installed. For Go development, you'll need to install Go itself (https://go.dev/doc/install).

#### Setting up MacOS
GNU command line tools are useful and should be installed on your system. This command installs the necessary packages:
```
brew install coreutils ed findutils gawk gnu-sed gnu-tar grep make jq go
```

### Running locally
Build the `bctl` image by `make build`. Then you may use various `bctl` commands to run against any Kubernetes cluster with [Blueprint Operator](https://github.com/MirantisContainers/blueprint-operator) installed.

## Submission Guidelines
Blueprint Operator follows a lightweight Pull Request process. When submitting a PR, answer a few basic questions around the type of change and steps to test, and you are well on your way to a PR approval.

## Your First Blueprint Operator Pull Request
Please Fork the project and create a branch to make your changes. Directly commit your changes to your branch and then, when ready to merge upstream, feel free to create a PR.

### Running Tests
As a good practice, locally running the integration tests is a good idea because during the CI Step (e.g., GitHub Actions), these will be run again as a quality gate.

#### Unit Tests
Unit test coverage is in place. Running `make test` will run all of the Unit Tests.

### Submitting PR
When you open a PR, there will be GitHub Actions that are run on your behalf. [GitHub Actions Config](./github/workflows/PR.yml)

Assuming everything passes in your branch, your PR is ready for review.
