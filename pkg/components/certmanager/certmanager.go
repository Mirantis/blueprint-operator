package certmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/blueprint-operator/pkg/components"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/blueprint-operator/pkg/utils"
)

const (
	namespaceCertManager  = "cert-manager"
	deploymentCAInjector  = "cert-manager-cainjector"
	deploymentCertManager = "cert-manager"
	deploymentWebhook     = "cert-manager-webhook"
)

// certManager is a component that manages cert manager in the cluster.
type certManager struct {
	client client.Client
	logger logr.Logger
}

// NewCertManagerComponent creates a new instance of the cert manager component.
func NewCertManagerComponent(client client.Client, logger logr.Logger) components.Component {
	return &certManager{
		client: client,
		logger: logger,
	}
}

// Name returns the name of the component.
func (c *certManager) Name() string {
	return "cert-manager"
}

// Install installs cert manager in the cluster.
func (c *certManager) Install(ctx context.Context) error {
	c.logger.Info("Installing cert manager")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := utils.CreateNamespaceIfNotExist(c.client, ctx, c.logger, consts.NamespaceBlueprintSystem); err != nil {
		return err
	}

	applier := kubernetes.NewApplier(c.logger, c.client)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(certManagerTemplate))); err != nil {
		return err
	}

	resources, err := kubernetes.NewManifestReader([]byte(certManagerTemplate)).ReadManifest()
	if err != nil {
		return err
	}

	var names []string
	for _, res := range resources {
		if res.GetKind() == "CustomResourceDefinition" {
			names = append(names, res.GetName())
		}
	}

	// wait for CRDs to be created
	if err = components.WaitForCRDs(ctx, c.client, c.logger, names); err != nil {
		return err
	}

	// Wait for all the deployments to be ready
	c.logger.Info("waiting for ca injector deployment to be ready")
	if err := utils.WaitForDeploymentReady(ctx, c.logger, c.client, deploymentCAInjector, consts.NamespaceBlueprintSystem); err != nil {
		return err
	}

	c.logger.Info("waiting for cert manager deployment to be ready")
	if err := utils.WaitForDeploymentReady(ctx, c.logger, c.client, deploymentCertManager, consts.NamespaceBlueprintSystem); err != nil {
		return err
	}

	c.logger.Info("waiting for webhook deployment to be ready")
	if err := utils.WaitForDeploymentReady(ctx, c.logger, c.client, deploymentWebhook, consts.NamespaceBlueprintSystem); err != nil {
		return err
	}

	c.logger.Info("finished installing cert manager")

	return nil
}

// Uninstall uninstalls cert manager from the cluster.
func (c *certManager) Uninstall(ctx context.Context) error {
	c.logger.Info("uninstalling cert manager")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(c.logger, c.client)
	reader := kubernetes.NewManifestReader([]byte(certManagerTemplate))
	objs, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	if err := applier.Delete(ctx, objs); err != nil {
		return err
	}

	c.logger.Info("Finished uninstalling cert manager")
	return nil
}

// CheckExists checks if cert manager is already installed in the cluster.
// This shall check both BOP specific as well as external installations.
func (c *certManager) CheckExists(ctx context.Context) (bool, error) {
	// First, we check if an external cert manager instance already exists in the cluster.
	exists, err := checkIfExternalCertManagerExists(ctx, c.client)
	if err != nil {
		return false, fmt.Errorf("failed to check if an external cert-manager installation already exists in the cluster")
	}
	if !exists {
		c.logger.Info("No external cert-manager installation detected.")

		key := client.ObjectKey{
			Namespace: consts.NamespaceBlueprintSystem,
			Name:      deploymentCertManager,
		}
		if err := c.client.Get(ctx, key, &v1.Deployment{}); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	c.logger.Info("External cert-manager installation detected.")
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
