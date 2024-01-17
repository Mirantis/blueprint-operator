package installation

import (
	"context"
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
	if err = waitForDeploymentReady(ctx, runtimeClient, logger, DeploymentCAInjector, NamespaceCertManager); err != nil {
		return err
	}

	logger.Info("waiting for cert manager deployment to be ready")
	if err = waitForDeploymentReady(ctx, runtimeClient, logger, DeploymentCertManager, NamespaceCertManager); err != nil {
		return err
	}

	logger.Info("waiting for webhook deployment to be ready")
	if err = waitForDeploymentReady(ctx, runtimeClient, logger, DeploymentWebhook, NamespaceCertManager); err != nil {
		return err
	}

	logger.Info("finished installing cert manager")

	return nil
}

func CheckIfCertManagerAlreadyExists(ctx context.Context, runtimeClient client.Client) (bool, error) {
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
