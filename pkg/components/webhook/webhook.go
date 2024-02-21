package webhook

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/components"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
)

const (
	serviceWebhook              = "boundless-operator-webhook-service"
	namespaceBoundlessSystem    = "boundless-system"
	deploymentControllerManager = "boundless-operator-controller-manager"
)

// webhook is a component that manages validation webhooks in the cluster.
type webhook struct {
	client client.Client
	logger logr.Logger
}

// NewWebhookComponent creates a new instance of the webhook component.
func NewWebhookComponent(client client.Client, logger logr.Logger) components.Component {
	return &webhook{
		client: client,
		logger: logger,
	}
}

// Name returns the name of the component
func (c *webhook) Name() string {
	return "webhook"
}

func (c *webhook) Install(ctx context.Context) error {
	var err error
	c.logger.Info("Installing validation webhooks")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(c.logger, c.client)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(webhookTemplate))); err != nil {
		return err
	}

	// Create certificate resources
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(certificateTemplate))); err != nil {
		c.logger.Info("failed to enable cert manager")
		return err
	}

	c.logger.Info("certificate resources created successfully")

	// Patch controller-manager deployment
	if err = patchControllerManagerDeployment(ctx, c.client, c.logger); err != nil {
		c.logger.Info("failed to patch existing controller-manager deployment ")
		return err
	}
	c.logger.Info("webhooks configured successfully in controller manager")

	return nil
}

// Uninstall uninstalls cert manager from the cluster.
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

// CheckExists checks if the helm controller exists in the cluster
func (c *webhook) CheckExists(ctx context.Context) (bool, error) {
	key := client.ObjectKey{
		Namespace: consts.NamespaceBoundlessSystem,
		Name:      serviceWebhook,
	}

	if err := c.client.Get(ctx, key, &coreV1.Service{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func patchControllerManagerDeployment(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	key := client.ObjectKey{
		Namespace: namespaceBoundlessSystem,
		Name:      deploymentControllerManager,
	}

	d := &v1.Deployment{}
	if err := runtimeClient.Get(ctx, key, d); err != nil {
		logger.Info("Failed to get deployment:%s, namespace: %s", deploymentControllerManager, namespaceBoundlessSystem)
		return err
	}

	port := coreV1.ContainerPort{
		ContainerPort: 9443,
		Name:          "webhook-server",
		Protocol:      "TCP",
	}

	vm := coreV1.VolumeMount{
		MountPath: "/tmp/k8s-webhook-server/serving-certs",
		Name:      "cert",
		ReadOnly:  true,
	}
	var mode int32 = 420
	secret := &coreV1.SecretVolumeSource{
		DefaultMode: &mode,
		SecretName:  "webhook-server-cert",
	}

	v := coreV1.Volume{
		Name: "cert",
		VolumeSource: coreV1.VolumeSource{
			Secret: secret,
		},
	}

	env := coreV1.EnvVar{
		Name:  "ENABLE_WEBHOOKS",
		Value: "true",
	}

	for i, _ := range d.Spec.Template.Spec.Containers {
		if d.Spec.Template.Spec.Containers[i].Name == "manager" {
			d.Spec.Template.Spec.Containers[i].Ports = append(d.Spec.Template.Spec.Containers[i].Ports, port)
			d.Spec.Template.Spec.Containers[i].VolumeMounts = append(d.Spec.Template.Spec.Containers[i].VolumeMounts, vm)
			d.Spec.Template.Spec.Containers[i].Env = []coreV1.EnvVar{env}
		}
	}

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
	if err := runtimeClient.Update(ctx, d); err != nil {
		logger.Info("Failed to update deployment:%s, namespace: %s", deploymentControllerManager, namespaceBoundlessSystem)
		return err
	}

	return nil
}
