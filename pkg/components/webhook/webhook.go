package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/blueprint-operator/pkg/components"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/blueprint-operator/pkg/utils"
)

const (
	serviceWebhook          = "blueprint-operator-webhook-service"
	webhookServerSecretName = "blueprint-webhook-server-cert"
)

// webhook is a component that manages validation webhooks in the cluster.
type webhook struct {
	client client.Client
	logger logr.Logger
}

type webhookConfig struct {
	Image string
}

// NewWebhookComponent creates a new instance of the webhook component.
func NewWebhookComponent(client client.Client, logger logr.Logger) components.Component {
	return &webhook{
		client: client,
		logger: logger,
	}
}

func (c *webhook) Images() []string {
	// webhook uses the same image as the controller manager, so no need to repeat it here
	return []string{}
}

// Name returns the name of the component
func (c *webhook) Name() string {
	return "webhook"
}

// Install installs webhooks in the cluster.
func (c *webhook) Install(ctx context.Context) error {
	c.logger.Info("Installing validation webhooks")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(c.logger, c.client)

	operatorImage, err := utils.GetOperatorImage(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to install webhooks: %w", err)
	}

	// Create certificate resources
	c.logger.V(2).Info("Creating certificate resources for webhook")
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(certificateTemplate))); err != nil {
		c.logger.Info("failed to create Certificate resources")
		return err
	}

	// Wait for the secret to be created before creating the webhook resources
	if err := utils.WaitForSecret(ctx, c.client, webhookServerSecretName, consts.NamespaceBlueprintSystem); err != nil {
		return err
	}

	cfg := webhookConfig{
		Image: operatorImage,
	}

	rendered, err := utils.ParseTemplate(webhookTemplate, cfg)
	if err != nil {
		return fmt.Errorf("failed to render webhook template: %w", err)
	}

	c.logger.V(2).Info("applying webhook resources with image: %s", operatorImage)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader(rendered.Bytes())); err != nil {
		return err
	}

	c.logger.Info("webhooks configured successfully")
	return nil
}

// Uninstall uninstalls validation webhooks from the cluster.
func (c *webhook) Uninstall(ctx context.Context) error {
	c.logger.Info("Uninstalling validation webhooks")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(c.logger, c.client)
	reader := kubernetes.NewManifestReader([]byte(webhookTemplate))
	objs, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	if err := applier.Delete(ctx, objs); err != nil {
		return err
	}

	reader = kubernetes.NewManifestReader([]byte(certificateTemplate))
	objs, err = reader.ReadManifest()
	if err != nil {
		return err
	}

	if err := applier.Delete(ctx, objs); err != nil {
		return err
	}

	c.logger.Info("Finished uninstalling webhook")

	return nil
}

// CheckExists checks if the webhook service exists in the cluster
func (c *webhook) CheckExists(ctx context.Context) (bool, error) {
	key := client.ObjectKey{
		Namespace: consts.NamespaceBlueprintSystem,
		Name:      serviceWebhook,
	}

	if err := c.client.Get(ctx, key, &corev1.Service{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
