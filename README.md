# Blueprint Operator 
## Overview
The Blueprint Operator is a comprehensive system designed to manage the lifecycle of Kubernetes clusters and their associated components through the use of a Blueprint. Originally known as Boundless, the Blueprint Operator has been rebranded to better align with its purpose and capabilities. It provides an efficient way to describe, configure, and manage the entire stack of a Kubernetes environment, from the infrastructure level to individual add-ons.

## Blueprint Specification
A Blueprint is a YAML-based specification that serves as a blueprint for defining a Kubernetes cluster and its add-on components. This specification is flexible, allowing users to define both the Kubernetes cluster and the add-ons that together constitute a fully functional product or environment.

- Kubernetes Cluster Definition: Users can define the specifics of their Kubernetes cluster, including the provider, version, and infrastructure configuration.
- Add-On Components: These are additional components such as Helm Charts or Operators that enhance the functionality of the Kubernetes cluster. Each add-on is described in detail within the Blueprint, including its configuration and deployment specifics.
## Goals
The primary goals of the Blueprint Operator are:

- Simplify Add-On Management: Provide a straightforward way to describe and configure Kubernetes add-ons.
- Lifecycle Management: Ensure the smooth management of the lifecycle of Kubernetes distributions and their components.
- Cluster Definition: Allow users to define and manage the entire Kubernetes cluster if desired.
## Add-Ons
Add-Ons are individual Kubernetes components that can be installed or updated as part of the Blueprint. They may include:

- Helm Charts: Pre-configured Kubernetes resources packaged together.
- Operators: Kubernetes applications that extend the API to manage complex software life cycles.
- Kubernetes Distro and Components Management: Ensuring the appropriate distribution and components are deployed and managed correctly.

## Blueprint Operator Features
The Blueprint Operator offers several key features to ensure reliable and efficient management of Kubernetes environments:

- Lifecycle Management: The operator ensures the lifecycle management of add-ons described in a Blueprint, whether they are Helm Charts or operator manifests.

- Secure Secret Management: Allows for the secure passing of secrets within Blueprints using environment variable substitution.

- Selective Installation: Supports installing only the add-ons if the Kubernetes infrastructure is already present, bypassing unnecessary steps.

- Idempotent Operations: Ensures idempotent operations for install, update, and delete actions, avoiding unintended side effects from repeated actions.

- Pre-Install Upgrade Checks: Performs checks before installation to ensure that components can be installed without issues, such as insufficient CPU, memory, or storage, or misconfigured Kubernetes settings. This includes a dry-run feature for generating a pre-install report.

- Comprehensive Logging: Generates detailed logs for every action taken, aiding in debugging and root cause analysis should any issues arise.

- Rollback Support: Supports automatic rollback after a configurable number of retries, providing resilience against failed deployments.

- Tooling Support: Includes a CLI for managing Blueprints and clusters, bundling all required dependencies (e.g., k0s CLI, Helm). The end user only needs to install the [Blueprint CLI](https://github.com/MirantisContainers/blueprint-cli), simplifying the setup process.  This is still in progress.

Here is an example [Blueprint YAML](https://mirantiscontainers.github.io/blueprint/docs/blueprint-reference/) .


The Blueprint Operator is a powerful tool for managing Kubernetes clusters and their components, providing users with the flexibility, control, and reliability needed to manage complex environments effectively.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/blueprint-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/blueprint-operator:tag
```

**Note**: If no IMG is specified, the latest image is used.

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
We welcome your help in building Blueprint! If you are interested, we invite you to check out the [Contributing Guide](./CONTRIBUTING.md) and the [Code of Conduct](./CODE-OF-CONDUCT.md).

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets
**NOTE:** Adding a new CR requires you to use operator-sdk(e.g. ./bin/operator-sdk create api --group blueprint --version v1alpha1 --kind <CR kind>). Currently this cannot be done using a make command 

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
