package certmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/components/helmcontroller"
	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
)

const (
	namespaceCertManager  = "cert-manager"
	deploymentCAInjector  = "cert-manager-cainjector"
	deploymentCertManager = "cert-manager"
	deploymentWebhook     = "cert-manager-webhook"
)

// Install installs cert manager in the cluster.
func Install(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	var err error
	logger.Info("Installing cert manager")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(logger, runtimeClient)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(CertManagerTemplate))); err != nil {
		return err
	}

	// Wait for all the deployments to be ready
	logger.Info("waiting for ca injector deployment to be ready")
	if err = utils.WaitForDeploymentReady(ctx, logger, runtimeClient, deploymentCAInjector, helmcontroller.NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("waiting for cert manager deployment to be ready")
	if err = utils.WaitForDeploymentReady(ctx, logger, runtimeClient, deploymentCertManager, helmcontroller.NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("waiting for webhook deployment to be ready")
	if err = utils.WaitForDeploymentReady(ctx, logger, runtimeClient, deploymentWebhook, helmcontroller.NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("finished installing cert manager")

	return nil
}

// Uninstall uninstalls cert manager from the cluster.
func Uninstall(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	logger.Info("uninstalling cert manager")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(logger, runtimeClient)
	reader := kubernetes.NewManifestReader([]byte(CertManagerTemplate))
	objs, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	if err := applier.Delete(ctx, objs); err != nil {
		return err
	}

	logger.Info("Finished uninstalling cert manager")
	return nil
}

// CheckExists checks if cert manager is already installed in the cluster.
// This shall check both BOP specific as well as external installations.
func CheckExists(ctx context.Context, runtimeClient client.Client, logger logr.Logger) (bool, error) {
	// First, we check if an external cert manager instance already exists in the cluster.
	exists, err := checkIfExternalCertManagerExists(ctx, runtimeClient)
	if err != nil {
		return false, fmt.Errorf("failed to check if an external cert-manager installation already exists in the cluster")
	}
	if !exists {
		logger.Info("No external cert-manager installation detected.")

		key := client.ObjectKey{
			Namespace: helmcontroller.NamespaceBoundlessSystem,
			Name:      deploymentCertManager,
		}
		if err := runtimeClient.Get(ctx, key, &v1.Deployment{}); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	logger.Info("External cert-manager installation detected.")
	return true, nil
}

// checkIfExternalCertManagerExists checks if an external cert manager instance already exists.
func checkIfExternalCertManagerExists(ctx context.Context, runtimeClient client.Client) (bool, error) {
	key := client.ObjectKey{
		Namespace: namespaceCertManager,
		Name:      deploymentCertManager,
	}
	if err := runtimeClient.Get(ctx, key, &v1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
