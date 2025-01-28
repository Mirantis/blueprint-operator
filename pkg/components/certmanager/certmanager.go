package certmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/blueprint-operator/internal/template"
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

	// images

	// CAInjectorImageTag is the tag of the cert-manager cainjector image
	CAInjectorImageTag = "v1.9.1"

	// ControllerImageTag is the tag of the cert-manager controller image
	ControllerImageTag = "v1.9.1"

	// WebhookImageTag is the tag of the cert-manager webhook image
	WebhookImageTag = "v1.9.1"

	caInjectorImage = "jetstack/cert-manager-cainjector:v1.9.1"
	controllerImage = "jetstack/cert-manager-controller:v1.9.1"
	webhookImage    = "jetstack/cert-manager-webhook:v1.9.1"
)

// certManager is a component that manages cert manager in the cluster.
type certManager struct {
	client        client.Client
	logger        logr.Logger
	imageRegistry string
}

// imageConfig holds the images for the cert manager components
type imageConfig struct {
	CAInjectorImage string
	ControllerImage string
	WebhookImage    string
}

func newImageConfig(registry string) imageConfig {
	if registry == "" {
		registry = consts.MirantisImageRegistry
	}

	return imageConfig{
		CAInjectorImage: fmt.Sprintf("%s/%s:%s", registry, caInjectorImage, CAInjectorImageTag),
		ControllerImage: fmt.Sprintf("%s/%s:%s", registry, controllerImage, ControllerImageTag),
		WebhookImage:    fmt.Sprintf("%s/%s:%s", registry, webhookImage, WebhookImageTag),
	}
}

// NewCertManagerComponent creates a new instance of the cert manager component.
func NewCertManagerComponent(client client.Client, logger logr.Logger, imageRegistry string) components.Component {
	return &certManager{
		client:        client,
		logger:        logger,
		imageRegistry: imageRegistry,
	}
}

// Name returns the name of the component.
func (c *certManager) Name() string {
	return "cert-manager"
}

// Images returns the images used by cert manager.
func (c *certManager) Images() []string {
	images := newImageConfig(c.imageRegistry)

	return []string{
		images.CAInjectorImage,
		images.ControllerImage,
		images.WebhookImage,
	}
}

func (c *certManager) renderManifest() ([]byte, error) {
	images := newImageConfig(c.imageRegistry)

	manifest, err := template.ParseTemplate(certManagerTemplate, images)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cert-manager manifest template: %w", err)
	}

	return manifest.Bytes(), nil
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

	certManagerManifest, err := c.renderManifest()
	if err != nil {
		return fmt.Errorf("unable to render cert-manager manifest: %w", err)
	}
	if err := applier.Apply(ctx, kubernetes.NewManifestReader(certManagerManifest)); err != nil {
		return err
	}

	resources, err := kubernetes.NewManifestReader(certManagerManifest).ReadManifest()
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

	certManagerManifest, err := c.renderManifest()
	if err != nil {
		return fmt.Errorf("unable to render cert-manager manifest: %w", err)
	}

	reader := kubernetes.NewManifestReader(certManagerManifest)
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
