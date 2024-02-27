package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
)

// BlueprintReconciler reconciles a Blueprint object
type BlueprintReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=blueprints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=blueprints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=blueprints/finalizers,verbs=update

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
	instance := &boundlessv1alpha1.Blueprint{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Blueprint instance not found. Ignoring since object must be deleted.", "Name", req.Name)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Blueprint instance", "Name", req.Name, "Requeue", true)
		return ctrl.Result{}, err
	}

	addonsToUninstall, err := r.getInstalledAddons(ctx, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, addonSpec := range instance.Spec.Components.Addons {
		if addonSpec.Namespace == "" {
			addonSpec.Namespace = instance.Namespace
		}

		logger.Info("Reconciling addonSpec", "Name", addonSpec.Name, "Spec.Namespace", addonSpec.Namespace)
		addon := addonResource(&addonSpec)
		err = r.createOrUpdateAddon(ctx, logger, addon)
		if err != nil {
			logger.Error(err, "Failed to reconcile addonSpec", "Name", addonSpec.Name, "Spec.Namespace", addonSpec.Namespace)
			return ctrl.Result{}, err
		}

		// if the addon is in the spec , we shouldn't uninstall it
		delete(addonsToUninstall, addon.GetName())
	}

	if len(addonsToUninstall) > 0 {
		err = r.deleteAddons(ctx, logger, addonsToUninstall)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getInstalledAddons returns a map of addons that are presently installed in the cluster
func (r *BlueprintReconciler) getInstalledAddons(ctx context.Context, logger logr.Logger) (map[string]boundlessv1alpha1.Addon, error) {
	allAddonsInCluster := &boundlessv1alpha1.AddonList{}
	if err := r.List(ctx, allAddonsInCluster); err != nil {
		return nil, err
	}

	logger.Info("existing addons are", "addonNames", allAddonsInCluster.Items)
	addonsToUninstall := make(map[string]boundlessv1alpha1.Addon)
	for _, addon := range allAddonsInCluster.Items {
		addonsToUninstall[addon.GetName()] = addon
	}

	return addonsToUninstall, nil
}

// deleteAddons deletes provided addonsToUninstall from the cluster
func (r *BlueprintReconciler) deleteAddons(ctx context.Context, logger logr.Logger, addonsToUninstall map[string]boundlessv1alpha1.Addon) error {
	for _, addon := range addonsToUninstall {
		logger.Info("Removing addon", "Name", addon.Name, "Namespace", addon.Spec.Namespace)
		if err := r.Delete(ctx, &addon, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to remove addon", "Name", addon.Name)
			return err
		}
	}

	return nil
}

func (r *BlueprintReconciler) createOrUpdateAddon(ctx context.Context, logger logr.Logger, addon *boundlessv1alpha1.Addon) error {
	err := utils.CreateNamespaceIfNotExist(r.Client, ctx, logger, addon.Spec.Namespace)
	if err != nil {
		return err
	}

	existing := &boundlessv1alpha1.Addon{}
	err = r.Get(ctx, client.ObjectKey{Name: addon.GetName(), Namespace: addon.GetNamespace()}, existing)
	if err != nil {
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
			err = r.Update(ctx, addon)
			if err != nil {
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
	err = r.Create(ctx, addon)
	if err != nil {
		return fmt.Errorf("failed to create add-on %s: %w", addon.GetName(), err)
	}
	return nil
}

func addonResource(spec *boundlessv1alpha1.AddonSpec) *boundlessv1alpha1.Addon {
	addon := &boundlessv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: consts.NamespaceBoundlessSystem,
		},
		Spec: boundlessv1alpha1.AddonSpec{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Kind:      spec.Kind,
			DryRun:    spec.DryRun,
		},
	}

	if spec.Chart != nil {
		addon.Spec.Chart = &boundlessv1alpha1.ChartInfo{
			Name:    spec.Chart.Name,
			Repo:    spec.Chart.Repo,
			Version: spec.Chart.Version,
			Set:     spec.Chart.Set,
			Values:  spec.Chart.Values,
		}
	}

	if spec.Manifest != nil {

		if spec.Manifest.Values == nil {
			addon.Spec.Manifest = &boundlessv1alpha1.ManifestInfo{
				URL:           spec.Manifest.URL,
				FailurePolicy: spec.Manifest.FailurePolicy,
				Timeout:       spec.Manifest.Timeout,
			}
		} else {
			addon.Spec.Manifest = &boundlessv1alpha1.ManifestInfo{
				URL:           spec.Manifest.URL,
				FailurePolicy: spec.Manifest.FailurePolicy,
				Timeout:       spec.Manifest.Timeout,
				Values: &boundlessv1alpha1.Values{
					Patches: spec.Manifest.Values.Patches,
					Images:  spec.Manifest.Values.Images,
				},
			}
		}
	}

	return addon
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlueprintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Blueprint{}).
		Complete(r)
}
