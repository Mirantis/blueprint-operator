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

func waitForDeploymentReady(ctx context.Context, runtimeClient client.Client, log logr.Logger, deploymentName, namespace string) error {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      deploymentName,
	}
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		d := &v1.Deployment{}
		err := runtimeClient.Get(ctx, key, d)
		if err != nil {
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
