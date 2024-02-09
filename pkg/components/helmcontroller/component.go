package helmcontroller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
)

const (
	// NamespaceBoundlessSystem is the namespace where all boundless components are installed
	NamespaceBoundlessSystem = "boundless-system"
	DeploymentHelmController = "helm-controller"
)

func Install(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	var err error
	logger.Info("Installing helm controller")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(logger, runtimeClient)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(HelmControllerTemplate))); err != nil {
		return err
	}

	// wait for helm controller to be ready
	logger.Info("Waiting for helm controller")
	if err = utils.WaitForDeploymentReady(ctx, logger, runtimeClient, DeploymentHelmController, NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("Finished installing helm controller")

	return nil
}

// Uninstall uninstalls cert manager from the cluster.
func Uninstall(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	logger.Info("Uninstalling helm controller")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(logger, runtimeClient)
	reader := kubernetes.NewManifestReader([]byte(HelmControllerTemplate))
	objs, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	if err := applier.Delete(ctx, objs); err != nil {
		return err
	}

	logger.Info("Finished uninstalling helm controller")
	return nil
}

// CheckExists checks if the helm controller exists in the cluster
func CheckExists(ctx context.Context, runtimeClient client.Client) (bool, error) {
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
