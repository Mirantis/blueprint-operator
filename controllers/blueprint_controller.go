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

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	boundlessCertmanager "github.com/mirantiscontainers/boundless-operator/pkg/components/certmanager"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
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

	err := r.reconcileAddons(ctx, logger, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileIssuers(ctx, logger, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileClusterIssuers(ctx, logger, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BlueprintReconciler) reconcileAddons(ctx context.Context, logger logr.Logger, instance *boundlessv1alpha1.Blueprint) error {
	addonsToUninstall, err := listInstalled[boundlessv1alpha1.Addon](ctx, logger, r.Client, &boundlessv1alpha1.AddonList{})
	if err != nil {
		return err
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
			return err
		}

		// if the addon is in the spec, we shouldn't uninstall it
		delete(addonsToUninstall, addon.GetName())
	}

	if len(addonsToUninstall) > 0 {
		err = deleteComponents(ctx, logger, r.Client, addonsToUninstall)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *BlueprintReconciler) reconcileIssuers(ctx context.Context, logger logr.Logger, instance *boundlessv1alpha1.Blueprint) error {
	issuersToUninstall, err := listInstalled[boundlessCertmanager.Issuer](ctx, logger, r.Client, &boundlessCertmanager.IssuerList{})
	if err != nil {
		return err
	}

	for _, issuerSpec := range instance.Spec.Components.CAs.Issuers {
		logger.Info("Reconciling issuerSpec", "Name", issuerSpec.Name, "Spec.Namespace", issuerSpec.Namespace)
		issuer := issuerResource(issuerSpec)

		err = r.createOrUpdateIssuer(ctx, logger, issuer)
		if err != nil {
			logger.Error(err, "Failed to reconcile issuerSpec", "Name", issuerSpec.Name, "Spec.Namespace", issuerSpec.Namespace)
			return err
		}

		// if the issuer is in the spec, we shouldn't uninstall it
		delete(issuersToUninstall, fmt.Sprintf("%s/%s", issuer.Namespace, issuer.Name))
	}

	if len(issuersToUninstall) > 0 {
		err = deleteComponents(ctx, logger, r.Client, issuersToUninstall)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *BlueprintReconciler) reconcileClusterIssuers(ctx context.Context, logger logr.Logger, instance *boundlessv1alpha1.Blueprint) error {
	clusterIssuersToUninstall, err := listInstalled[boundlessCertmanager.ClusterIssuer](ctx, logger, r.Client, &boundlessCertmanager.ClusterIssuerList{})
	if err != nil {
		return err
	}

	for _, clusterIssuerSpec := range instance.Spec.Components.CAs.ClusterIssuers {
		logger.Info("Reconciling clusterIssuerSpec", "Name", clusterIssuerSpec.Name)
		clusterIssuer := clusterIssuerResource(clusterIssuerSpec)

		err = r.createOrUpdateClusterIssuer(ctx, logger, clusterIssuer)
		if err != nil {
			logger.Error(err, "Failed to reconcile clusterIssuerSpec", "Name", clusterIssuerSpec.Name)
			return err
		}

		// if the clusterIssuer is in the spec, we shouldn't uninstall it
		delete(clusterIssuersToUninstall, clusterIssuer.Name)
	}

	if len(clusterIssuersToUninstall) > 0 {
		err = deleteComponents(ctx, logger, r.Client, clusterIssuersToUninstall)
		if err != nil {
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

type BlueprintComponent interface {
	GetComponentName() string
	GetComponentNamespace() string
	GetObject() client.Object
}

type BlueprintComponentList[T BlueprintComponent] interface {
	GetItems() []T
	GetObjectList() client.ObjectList
}

func listInstalled[T BlueprintComponent](ctx context.Context, logger logr.Logger, apiClient client.Client, list BlueprintComponentList[T]) (map[string]T, error) {
	if err := apiClient.List(ctx, list.GetObjectList()); err != nil {
		return nil, err
	}

	logger.Info("existing items are", "names", list)
	itemsToUninstall := make(map[string]T)

	for _, item := range list.GetItems() {
		itemsToUninstall[item.GetComponentName()] = item
	}

	return itemsToUninstall, nil
}

func issuerResource(issuer boundlessv1alpha1.Issuer) *certmanager.Issuer {
	return &certmanager.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      issuer.Name,
			Namespace: issuer.Namespace,
		},
		Spec: issuer.Spec,
	}
}

func clusterIssuerResource(clusterIssuer boundlessv1alpha1.ClusterIssuer) *certmanager.ClusterIssuer {
	return &certmanager.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterIssuer.Name,
			Namespace: consts.NamespaceBoundlessSystem,
		},
		Spec: clusterIssuer.Spec,
	}
}

func (r *BlueprintReconciler) createOrUpdateIssuer(ctx context.Context, logger logr.Logger, issuer *certmanager.Issuer) error {
	err := utils.CreateNamespaceIfNotExist(r.Client, ctx, logger, issuer.Namespace)
	if err != nil {
		return err
	}

	existing := &certmanager.Issuer{}
	err = r.Get(ctx, client.ObjectKey{Name: issuer.GetName(), Namespace: issuer.GetNamespace()}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if existing.Name != "" {
		logger.Info("Issuer already exists. Updating", "Name", existing.Name, "Spec.Namespace", existing.Namespace)
		issuer.SetResourceVersion(existing.GetResourceVersion())
		issuer.SetFinalizers(existing.GetFinalizers())
		err = r.Update(ctx, issuer)
		if err != nil {
			return fmt.Errorf("failed to update issuer %s: %w", existing.Name, err)
		}
		return nil
	}

	logger.Info("Creating issuer", "Name", issuer.Name, "Spec.Namespace", issuer.Namespace)
	err = r.Create(ctx, issuer)
	if err != nil {
		return fmt.Errorf("failed to create issuer %s: %w", issuer.Name, err)
	}
	return nil
}

func (r *BlueprintReconciler) createOrUpdateClusterIssuer(ctx context.Context, logger logr.Logger, clusterIssuer *certmanager.ClusterIssuer) error {
	existing := &certmanager.ClusterIssuer{}
	err := r.Get(ctx, client.ObjectKey{Name: clusterIssuer.GetName(), Namespace: consts.NamespaceBoundlessSystem}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if existing.Name != "" {
		logger.Info("ClusterIssuer already exists. Updating", "Name", existing.Name)

		clusterIssuer.SetResourceVersion(existing.GetResourceVersion())
		clusterIssuer.SetFinalizers(existing.GetFinalizers())
		err = r.Update(ctx, clusterIssuer)
		if err != nil {
			return fmt.Errorf("failed to update clusterIssuer %s: %w", existing.Name, err)
		}
		return nil
	}

	logger.Info("Creating clusterIssuer", "Name", clusterIssuer.Name)
	err = r.Create(ctx, clusterIssuer)
	if err != nil {
		return fmt.Errorf("failed to create clusterIssuer %s: %w", clusterIssuer.Name, err)
	}
	return nil
}

func deleteComponents[T BlueprintComponent](ctx context.Context, logger logr.Logger, apiClient client.Client, componentsToUninstall map[string]T) error {
	for _, component := range componentsToUninstall {
		logger.Info("Removing object", "Name", component.GetComponentName(), "Namespace", component.GetComponentNamespace())
		if err := apiClient.Delete(ctx, component.GetObject(), client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to remove object", "Name", component.GetComponentName())
			return err
		}
	}

	return nil
}
