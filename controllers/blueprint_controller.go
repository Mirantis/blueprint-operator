package controllers

import (
	"context"
	"fmt"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/pkg/utils"

	blueprintv1alpha1 "github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
)

// BlueprintReconciler reconciles a Blueprint object
type BlueprintReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=blueprints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=blueprints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=blueprints/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Blueprint object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BlueprintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile request on Blueprint instance", "Name", req.Name)
	instance := &blueprintv1alpha1.Blueprint{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Blueprint instance not found. Ignoring since object must be deleted.", "Name", req.Name)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Blueprint instance", "Name", req.Name, "Requeue", true)
		return ctrl.Result{}, err
	}

	err := r.reconcileAddons(ctx, logger, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconcileObjects(ctx, logger, r.Client,
		convertToObjects(instance.Spec.Resources.CertManagement.Issuers, issuerObject), listIssuers)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to reconcile Issuers: %w", err)
	}

	err = reconcileObjects(ctx, logger, r.Client,
		convertToObjects(instance.Spec.Resources.CertManagement.ClusterIssuers, clusterIssuerObject), listClusterIssuers)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to reconcile ClusterIssuers: %w", err)
	}

	err = reconcileObjects(ctx, logger, r.Client,
		convertToObjects(instance.Spec.Resources.CertManagement.Certificates, certificateObject), listCertificates)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to reconcile Resources: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *BlueprintReconciler) reconcileAddons(ctx context.Context, logger logr.Logger, instance *blueprintv1alpha1.Blueprint) error {
	addonsToUninstall, err := r.getInstalledAddons(ctx, logger)
	if err != nil {
		return err
	}

	for _, addonSpec := range instance.Spec.Components.Addons {
		if addonSpec.Namespace == "" {
			addonSpec.Namespace = instance.Namespace
		}

		if !addonSpec.Enabled {
			// No create/update the addon if it is not enabled
			continue
		}

		logger.Info("Reconciling addonSpec", "Name", addonSpec.Name, "Spec.Namespace", addonSpec.Namespace)
		addon := addonResource(&addonSpec)
		err = r.createOrUpdateAddon(ctx, logger, addon)
		if err != nil {
			logger.Error(err, "Failed to reconcile addonSpec", "Name", addonSpec.Name, "Spec.Namespace", addonSpec.Namespace)
			return err
		}

		// if the addon is in the spec , we shouldn't uninstall it
		delete(addonsToUninstall, addon.GetName())
	}

	if len(addonsToUninstall) > 0 {
		err = r.deleteAddons(ctx, logger, addonsToUninstall)
		if err != nil {
			return err
		}
	}

	return nil
}

// getInstalledAddons returns a map of addons that are presently installed in the cluster
func (r *BlueprintReconciler) getInstalledAddons(ctx context.Context, logger logr.Logger) (map[string]blueprintv1alpha1.Addon, error) {
	allAddonsInCluster := &blueprintv1alpha1.AddonList{}
	if err := r.List(ctx, allAddonsInCluster); err != nil {
		return nil, err
	}

	logger.Info("existing addons are", "addonNames", allAddonsInCluster.Items)
	addonsToUninstall := make(map[string]blueprintv1alpha1.Addon)
	for _, addon := range allAddonsInCluster.Items {
		addonsToUninstall[addon.GetName()] = addon
	}

	return addonsToUninstall, nil
}

// deleteAddons deletes provided addonsToUninstall from the cluster
func (r *BlueprintReconciler) deleteAddons(ctx context.Context, logger logr.Logger, addonsToUninstall map[string]blueprintv1alpha1.Addon) error {
	for _, addon := range addonsToUninstall {
		logger.Info("Removing addon", "Name", addon.Name, "Namespace", addon.Spec.Namespace)
		if err := r.Delete(ctx, &addon, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to remove addon", "Name", addon.Name)
			return err
		}
	}

	return nil
}

func (r *BlueprintReconciler) createOrUpdateAddon(ctx context.Context, logger logr.Logger, addon *blueprintv1alpha1.Addon) error {
	existing := &blueprintv1alpha1.Addon{}
	if err := r.Get(ctx, client.ObjectKey{Name: addon.GetName(), Namespace: addon.GetNamespace()}, existing); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if existing.Name != "" {
		logger.Info("Add-on already exists. Updating", "Name", existing.Name, "Spec.Namespace", existing.Spec.Namespace)

		if existing.Spec.Namespace == addon.Spec.Namespace {
			addon.SetResourceVersion(existing.GetResourceVersion())
			// TODO : Copy all the fields from the existing
			addon.SetFinalizers(existing.GetFinalizers())
			if err := r.Update(ctx, addon); err != nil {
				return fmt.Errorf("failed to update add-on %s: %w", existing.Name, err)
			}
			return nil
		} else {
			// the addon spec has moved namespaces, we need to delete and re-create it
			logger.Info("Addon has moved namespaces, deleting old version of add on",
				"Name", addon.Name,
				"Old Namespace", existing.Spec.Namespace,
				"New Namespace", addon.Spec.Namespace)
			if err := r.Delete(ctx, existing, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				logger.Error(err, "Failed to remove old version of addon", "Name", existing.Name)
				return err
			}
		}
	}

	logger.Info("Creating add-on", "Name", addon.GetName(), "Spec.Namespace", addon.Spec.Namespace)
	if err := r.Create(ctx, addon); err != nil {
		return fmt.Errorf("failed to create add-on %s: %w", addon.GetName(), err)
	}
	return nil
}

func addonResource(spec *blueprintv1alpha1.AddonSpec) *blueprintv1alpha1.Addon {
	addon := &blueprintv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: consts.NamespaceBlueprintSystem,
		},
		Spec: blueprintv1alpha1.AddonSpec{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Kind:      spec.Kind,
			DryRun:    spec.DryRun,
		},
	}

	if spec.Chart != nil {
		addon.Spec.Chart = &blueprintv1alpha1.ChartInfo{
			Name:      spec.Chart.Name,
			Repo:      spec.Chart.Repo,
			Version:   spec.Chart.Version,
			Set:       spec.Chart.Set,
			Values:    spec.Chart.Values,
			DependsOn: spec.Chart.DependsOn,
		}
	}

	if spec.Manifest != nil {

		if spec.Manifest.Values == nil {
			addon.Spec.Manifest = &blueprintv1alpha1.ManifestInfo{
				URL:           spec.Manifest.URL,
				FailurePolicy: spec.Manifest.FailurePolicy,
				Timeout:       spec.Manifest.Timeout,
			}
		} else {
			addon.Spec.Manifest = &blueprintv1alpha1.ManifestInfo{
				URL:           spec.Manifest.URL,
				FailurePolicy: spec.Manifest.FailurePolicy,
				Timeout:       spec.Manifest.Timeout,
				Values: &blueprintv1alpha1.Values{
					Patches: spec.Manifest.Values.Patches,
					Images:  spec.Manifest.Values.Images,
				},
			}
		}
	}

	return addon
}

func issuerObject(issuer blueprintv1alpha1.Issuer) client.Object {
	return &certmanager.Issuer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cert-manager.io/v1",
			Kind:       "Issuer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      issuer.Name,
			Namespace: issuer.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "blueprint-operator",
			},
		},
		Spec: issuer.Spec,
	}
}

func clusterIssuerObject(issuer blueprintv1alpha1.ClusterIssuer) client.Object {
	return &certmanager.ClusterIssuer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cert-manager.io/v1",
			Kind:       "ClusterIssuer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: issuer.Name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "blueprint-operator",
			},
		},
		Spec: issuer.Spec,
	}
}

func certificateObject(certificate blueprintv1alpha1.Certificate) client.Object {
	return &certmanager.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cert-manager.io/v1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificate.Name,
			Namespace: certificate.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "blueprint-operator",
			},
		},
		Spec: certificate.Spec,
	}
}

func listIssuers(ctx context.Context, apiClient client.Client) ([]client.Object, error) {
	issuerList := &certmanager.IssuerList{}
	if err := apiClient.List(ctx, issuerList); err != nil {
		return nil, err
	}

	return convertToObjects(utils.PointSlice(issuerList.Items), directConverter[*certmanager.Issuer]), nil
}

func listClusterIssuers(ctx context.Context, apiClient client.Client) ([]client.Object, error) {
	clusterIssuerList := &certmanager.ClusterIssuerList{}
	if err := apiClient.List(ctx, clusterIssuerList); err != nil {
		return nil, err
	}

	return convertToObjects(utils.PointSlice(clusterIssuerList.Items), directConverter[*certmanager.ClusterIssuer]), nil
}

func listCertificates(ctx context.Context, apiClient client.Client) ([]client.Object, error) {
	certificateList := &certmanager.CertificateList{}
	if err := apiClient.List(ctx, certificateList); err != nil {
		return nil, err
	}

	return convertToObjects(utils.PointSlice(certificateList.Items), directConverter[*certmanager.Certificate]), nil
}

func directConverter[T client.Object](object T) client.Object {
	return object
}

func convertToObjects[T any](items []T, converter func(T) client.Object) []client.Object {
	objects := make([]client.Object, len(items))
	for i, item := range items {
		objects[i] = converter(item)
	}
	return objects
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlueprintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&blueprintv1alpha1.Blueprint{}).
		Complete(r)
}
