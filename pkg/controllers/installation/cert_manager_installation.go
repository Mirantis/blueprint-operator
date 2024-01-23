package installation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
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
	CRDBlueprint                = "blueprints.boundless.mirantis.com"
	CRDIngress                  = "ingresses.boundless.mirantis.com"
	CRDManifest                 = "manifests.boundless.mirantis.com"
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
	/*if err = patchExistingCRDs(ctx, runtimeClient, logger, CRDAddon); err != nil {
		logger.Info("failed to patch existing CRD")
		return err
	}
	if err = patchExistingCRDs(ctx, runtimeClient, logger, CRDBlueprint); err != nil {
		logger.Info("failed to patch existing CRD")
		return err
	}
	if err = patchExistingCRDs(ctx, runtimeClient, logger, CRDIngress); err != nil {
		logger.Info("failed to patch existing CRD")
		return err
	}
	if err = patchExistingCRDs(ctx, runtimeClient, logger, CRDManifest); err != nil {
		logger.Info("failed to patch existing CRD")
		return err
	}*/
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(manifests.CRDPatchTemplate))); err != nil {
		logger.Info("failed to patch crds")
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

func patchExistingCRDs(ctx context.Context, runtimeClient client.Client, logger logr.Logger, crd string) error {
	key := client.ObjectKey{
		Namespace: NamespaceBoundlessSystem,
		Name:      crd,
	}

	d := &apiextensions.CustomResourceDefinition{}
	if err := runtimeClient.Get(ctx, key, d); err != nil {
		logger.Info("Failed to get crd")
		return err
	}
	annotations := map[string]string{
		"cert-manager.io/inject-ca-from":        "boundless-system/boundless-operator-serving-cert",
		"controller-gen.kubebuilder.io/version": "v0.11.1",
	}

	path := "/convert"
	webhookClientConfig := &apiextensions.WebhookClientConfig{
		Service: &apiextensions.ServiceReference{
			Name:      "boundless-operator-webhook-service",
			Namespace: "boundless-system",
			Path:      &path,
		},
	}

	conversionResource := &apiextensions.CustomResourceConversion{
		Strategy:                 "Webhook",
		WebhookClientConfig:      webhookClientConfig,
		ConversionReviewVersions: []string{"v1"},
	}

	d.ObjectMeta.Annotations = annotations
	d.Spec.Conversion = conversionResource

	if err := runtimeClient.Update(ctx, d); err != nil {
		logger.Info("Failed to update crd")
		return err
	}

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
