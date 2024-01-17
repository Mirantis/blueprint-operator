package installation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/controllers/installation/manifests"
	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
)

const (
	// NamespaceBoundlessSystem is the namespace where all boundless components are installed
	NamespaceBoundlessSystem = "boundless-system"
	DeploymentHelmController = "helm-controller"
)

func InstallHelmController(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	var err error

	logger.Info("installing helm controller")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(logger, runtimeClient)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(manifests.HelmControllerTemplate))); err != nil {
		return err
	}

	// wait for helm controller to be ready
	logger.Info("waiting for helm controller")
	if err = waitForDeploymentReady(ctx, runtimeClient, logger, DeploymentHelmController, NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("finished installing helm controller")

	return nil
}

func CheckHelmControllerExists(ctx context.Context, runtimeClient client.Client) (bool, error) {
	key := client.ObjectKey{
		Namespace: NamespaceBoundlessSystem,
		Name:      DeploymentHelmController,
	}
	if err := runtimeClient.Get(ctx, key, &v1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func waitForDeploymentReady(ctx context.Context, runtimeClient client.Client, log logr.Logger, deploymentName, namespace string) error {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      deploymentName,
	}
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		d := &v1.Deployment{}
		if err := runtimeClient.Get(ctx, key, d); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if d.Status.AvailableReplicas == d.Status.Replicas {
			// Expected replicas active
			return true, nil
		}
		log.V(1).Info(fmt.Sprintf("waiting for deployment %s to %d replicas, currently at %d", deploymentName, d.Status.Replicas, d.Status.AvailableReplicas))
		return false, nil
	})
}
