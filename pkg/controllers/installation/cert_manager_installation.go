package installation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/controllers/installation/manifests"
	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
)

const (
	NamespaceCertManager  = "cert-manager"
	DeploymentCAInjector  = "cert-manager-cainjector"
	DeploymentCertManager = "cert-manager"
	DeploymentWebhook     = "cert-manager-webhook"
)

func InstallCertManager(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	var err error

	logger.Info("installing cert manager")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(logger, runtimeClient)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(manifests.CertManagerTemplate))); err != nil {
		return err
	}

	// Wait for all the deployments to be ready
	logger.Info("waiting for ca injector deployment to be ready")
	if err = waitForDeploymentReady(ctx, runtimeClient, logger, DeploymentCAInjector, NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("waiting for cert manager deployment to be ready")
	if err = waitForDeploymentReady(ctx, runtimeClient, logger, DeploymentCertManager, NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("waiting for webhook deployment to be ready")
	if err = waitForDeploymentReady(ctx, runtimeClient, logger, DeploymentWebhook, NamespaceBoundlessSystem); err != nil {
		return err
	}

	logger.Info("finished installing cert manager")

	return nil
}

// CheckIfCertManagerAlreadyExists checks if cert manager is already installed in the cluster.
// This shall check both BOP specific as well as external installations.
func CheckIfCertManagerAlreadyExists(ctx context.Context, runtimeClient client.Client, logger logr.Logger) (bool, error) {
	// First, we check if an external cert manager instance already exists in the cluster.
	exists, err := checkIfExternalCertManagerExists(ctx, runtimeClient)
	if err != nil {
		return false, fmt.Errorf("failed to check if an external cert manager installation already exists in the cluster")
	}
	if !exists {
		logger.Info("No external cert-manager installation detected.")

		// Check BOP-specific installation
		key := client.ObjectKey{
			Namespace: NamespaceBoundlessSystem,
			Name:      DeploymentCertManager,
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
		Namespace: NamespaceCertManager,
		Name:      DeploymentCertManager,
	}
	if err := runtimeClient.Get(ctx, key, &v1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
