package installation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/controllers/installation/manifests"
	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
)

const (
	NamespaceCertManager        = "cert-manager"
	DeploymentCAInjector        = "cert-manager-cainjector"
	DeploymentCertManager       = "cert-manager"
	DeploymentWebhook           = "cert-manager-webhook"
	CRDAddon                    = "addons.boundless.mirantis.com"
	DeploymentControllerManager = "boundless-operator-controller-manager"
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

	// Now, make changes in the configuration
	if err = patchExistingCRDs(ctx, runtimeClient, logger); err != nil {
		logger.Info("failed to patch existing CRDs ")
		return err
	}
	// Enable webhook
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(manifests.WebhookConfigTemplate))); err != nil {
		logger.Info("failed to create webhook")
		return err
	}

	// Enable cert-manager
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(manifests.CertManagerConfigTemplate))); err != nil {
		logger.Info("failed to enable cert manager")
		return err
	}

	// Patch controller-manager deployment
	if err = patchControllerManagerWebhook(ctx, runtimeClient, logger); err != nil {
		logger.Info("failed to patch existing CRDs ")
		return err
	}

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

func patchExistingCRDs(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	/*key := client.ObjectKey{
		Namespace: namespace,
		Name:      deploymentName,
	}
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		d := &v1.Deployment{}
		if err := runtimeClient.Get(ctx, key, d); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if d.Status.AvailableReplicas == d.Status.Replicas {
			// Expected replicas active
			return true, nil
		}
		log.V(1).Info(fmt.Sprintf("waiting for deployment %s to %d replicas, currently at %d", deploymentName, d.Status.Replicas, d.Status.AvailableReplicas))
		return false, nil
	})*/

	return nil
}

func patchControllerManagerWebhook(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	key := client.ObjectKey{
		Namespace: NamespaceBoundlessSystem,
		Name:      DeploymentControllerManager,
	}

	d := &v1.Deployment{}
	if err := runtimeClient.Get(ctx, key, d); err != nil {
		logger.Info("Failed to get deployment:%s, namespace: %s", DeploymentControllerManager, NamespaceBoundlessSystem)
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
			d.Spec.Template.Spec.Containers[i].Env = append(d.Spec.Template.Spec.Containers[i].Env, env)
		}
	}

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
	if err := runtimeClient.Update(ctx, d); err != nil {
		logger.Info("Failed to update deployment:%s, namespace: %s", DeploymentControllerManager, NamespaceBoundlessSystem)
		return err
	}

	return nil
}
