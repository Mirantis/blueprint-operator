package certmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"github.com/mirantiscontainers/boundless-operator/pkg/components"
	"github.com/mirantiscontainers/boundless-operator/pkg/components/webhook"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespaceCertManager        = "cert-manager"
	deploymentCAInjector        = "cert-manager-cainjector"
	deploymentCertManager       = "cert-manager"
	deploymentWebhook           = "cert-manager-webhook"
	deploymentControllerManager = "boundless-operator-controller-manager"
	namespaceBoundlessSystem    = "boundless-system"
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
	var err error
	c.logger.Info("Installing cert manager")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(c.logger, c.client)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(certManagerTemplate))); err != nil {
		return err
	}

	// Wait for all the deployments to be ready
	c.logger.Info("waiting for ca injector deployment to be ready")
	if err = utils.WaitForDeploymentReady(ctx, c.logger, c.client, deploymentCAInjector, consts.NamespaceBoundlessSystem); err != nil {
		return err
	}

	c.logger.Info("waiting for cert manager deployment to be ready")
	if err = utils.WaitForDeploymentReady(ctx, c.logger, c.client, deploymentCertManager, consts.NamespaceBoundlessSystem); err != nil {
		return err
	}

	c.logger.Info("waiting for webhook deployment to be ready")
	if err = utils.WaitForDeploymentReady(ctx, c.logger, c.client, deploymentWebhook, consts.NamespaceBoundlessSystem); err != nil {
		return err
	}

	c.logger.Info("finished installing cert manager")

	// Now, make changes in the configuration to enable webhooks
	// Enable webhook
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(webhook.WebhookTemplate))); err != nil {
		c.logger.Info("failed to create webhook")
		return err
	}

	c.logger.Info("webhook enabled successfully")

	// Create certificate resources
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(CertificateTemplate))); err != nil {
		c.logger.Info("failed to enable cert manager")
		return err
	}

	c.logger.Info("certificate resources created successfully")

	// Patch controller-manager deployment
	if err = patchControllerManagerWebhook(ctx, c.client, c.logger); err != nil {
		c.logger.Info("failed to patch existing controller-manager deployment ")
		return err
	}
	c.logger.Info("webhooks configured successfully in controller manager")

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
			Namespace: consts.NamespaceBoundlessSystem,
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

func patchControllerManagerWebhook(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
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
