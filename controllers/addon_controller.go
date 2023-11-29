package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	"github.com/mirantis/boundless-operator/pkg/event"
	"github.com/mirantis/boundless-operator/pkg/helm"
	"github.com/mirantis/boundless-operator/pkg/manifest"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	kindManifest            = "manifest"
	kindChart               = "chart"
	BoundlessNamespace      = "boundless-system"
	addonHelmchartFinalizer = "boundless.mirantis.com/helmchart-finalizer"
	addonManifestFinalizer  = "boundless.mirantis.com/manifest-finalizer"
)

// AddonReconciler reconciles a Addon object
type AddonReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=addons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=addons/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=addons/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Addon object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *AddonReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	_ = log.FromContext(ctx)

	logger := log.FromContext(ctx)
	logger.Info("Reconcile request on Addon instance", "Name", req.Name)

	instance := &boundlessv1alpha1.Addon{}
	err = r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		msg := "failed to get MkeAddon instance"
		if errors.IsNotFound(err) {
			// Ignore request.
			logger.Info(msg, "Name", req.Name, "Requeue", false)
			return ctrl.Result{}, nil
		}
		logger.Error(err, msg, "Name", req.Namespace, "Requeue", true)
		return ctrl.Result{}, err
	}

	logger.Info("Reconcile Addon Generation id", "GenID", instance.Generation)

	var kind string
	if strings.EqualFold(kindChart, instance.Spec.Kind) {
		kind = kindChart
	} else if strings.EqualFold(kindManifest, instance.Spec.Kind) {
		kind = kindManifest
	} else {
		kind = instance.Spec.Kind
	}

	// @TODO: Update addon status only once per reconcile; React to Statuses of HelmChart / Manifests

	switch kind {
	case kindChart:
		if instance.Spec.Chart == nil {
			logger.Info("Chart info is missing")
			return ctrl.Result{Requeue: false}, fmt.Errorf("chart info is missing: %w", err)
		}
		chart := helm.Chart{
			Name:    instance.Spec.Chart.Name,
			Repo:    instance.Spec.Chart.Repo,
			Version: instance.Spec.Chart.Version,
			Set:     instance.Spec.Chart.Set,
			Values:  instance.Spec.Chart.Values,
		}

		logger.Info("Reconciler instance details", "Name", instance.Spec.Chart.Name)

		hc := helm.NewHelmChartController(r.Client, logger)

		if instance.ObjectMeta.DeletionTimestamp.IsZero() {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			if !controllerutil.ContainsFinalizer(instance, addonHelmchartFinalizer) {
				controllerutil.AddFinalizer(instance, addonHelmchartFinalizer)
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
			}
		} else {
			// The object is being deleted
			if controllerutil.ContainsFinalizer(instance, addonHelmchartFinalizer) {
				// our finalizer is present, so lets delete the helm chart
				if err := hc.DeleteHelmChart(chart, instance.Spec.Namespace); err != nil {
					// if fail to delete the helm chart here, return with error
					// so that it can be retried
					r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedDelete, "Failed to Delete Chart Addon %s/%s: %s", instance.Spec.Namespace, instance.Name, err)
					return ctrl.Result{}, err
				}

				// remove our finalizer from the list and update it.
				controllerutil.RemoveFinalizer(instance, addonHelmchartFinalizer)
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
			}

			// Stop reconciliation as the item is being deleted
			return ctrl.Result{}, nil
		}

		logger.Info("Creating Addon HelmChart resource", "Name", chart.Name, "Version", chart.Version)
		if err := hc.CreateHelmChart(chart, instance.Spec.Namespace); err != nil {
			logger.Error(err, "failed to install addon", "Name", chart.Name, "Version", chart.Version)
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "Failed to Create Chart Addon %s/%s : %s", instance.Spec.Namespace, instance.Name, err)
			r.updateStatus(ctx, logger, req.NamespacedName, boundlessv1alpha1.TypeComponentUnhealthy, "Failed to Create HelmChart")
			return ctrl.Result{Requeue: true}, err
		}
		r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Chart Addon %s/%s", instance.Spec.Namespace, instance.Name)
		r.updateStatus(ctx, logger, req.NamespacedName, boundlessv1alpha1.TypeComponentAvailable, "Chart Addon Created")

	case kindManifest:
		if instance.Spec.Manifest == nil {
			logger.Info("Manifest info is missing")
			return ctrl.Result{Requeue: false}, fmt.Errorf("manifest info is missing: %w", err)
		}
		mc := manifest.NewManifestController(r.Client, logger)

		if instance.ObjectMeta.DeletionTimestamp.IsZero() {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			if !controllerutil.ContainsFinalizer(instance, addonManifestFinalizer) {
				controllerutil.AddFinalizer(instance, addonManifestFinalizer)
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		} else {
			// The object is being deleted
			if controllerutil.ContainsFinalizer(instance, addonManifestFinalizer) {
				// our finalizer is present, so lets delete the helm chart
				if err := mc.DeleteManifest(BoundlessNamespace, instance.Spec.Name, instance.Spec.Manifest.URL); err != nil {
					// if fail to delete the manifest here, return with error
					// so that it can be retried
					r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedDelete, "Failed to Delete Manifest Addon %s/%s : %s", instance.Spec.Namespace, instance.Name, err)
					r.updateStatus(ctx, logger, req.NamespacedName, boundlessv1alpha1.TypeComponentUnhealthy, "Failed to Cleanup Manifest")
					return ctrl.Result{}, err
				}

				// remove our finalizer from the list and update it.
				controllerutil.RemoveFinalizer(instance, addonManifestFinalizer)
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
			}

			// Stop reconciliation as the item is being deleted
			return ctrl.Result{}, nil
		}

		err = mc.CreateManifest(BoundlessNamespace, instance.Spec.Name, instance.Spec.Manifest.URL)
		if err != nil {
			logger.Error(err, "failed to install addon via manifest", "URL", instance.Spec.Manifest.URL)
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "Failed to Create Manifest Addon %s/%s : %s", instance.Spec.Namespace, instance.Name, err)
			r.updateStatus(ctx, logger, req.NamespacedName, boundlessv1alpha1.TypeComponentUnhealthy, "Failed to Create Manifest")
			return ctrl.Result{Requeue: true}, err
		}

		r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Manifest Addon %s/%s", instance.Spec.Namespace, instance.Name)
		r.updateStatus(ctx, logger, req.NamespacedName, boundlessv1alpha1.TypeComponentAvailable, "Manifest Addon Created")

	default:
		logger.Info("Unknown AddOn kind", "Kind", instance.Spec.Kind)
		return ctrl.Result{Requeue: false}, fmt.Errorf("Unknown AddOn Kind: %w", err)
	}

	logger.Info("Finished reconcile request on MkeAddon instance", "Name", req.Name)
	return ctrl.Result{Requeue: false}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Addon{}).
		Complete(r)
}

func (r *AddonReconciler) updateStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, conditionTypeToApply boundlessv1alpha1.StatusType, reasonToApply string, messageToApply ...string) error {
	addon := &boundlessv1alpha1.Addon{}
	err := r.Get(ctx, namespacedName, addon)
	if err != nil {
		logger.Error(err, "Failed to get addon to update status")
		return err
	}

	if addon.Status.Type == conditionTypeToApply && addon.Status.Reason == reasonToApply {
		// avoid infinite reconciliation loops
		logger.Info("No updates to status needed")
		return nil
	}

	logger.Info("Update status for addon", "Name", addon.Name)

	patch := client.MergeFrom(addon.DeepCopy())
	addon.Status.Type = conditionTypeToApply
	addon.Status.Reason = reasonToApply
	if len(messageToApply) > 0 {
		addon.Status.Message = messageToApply[0]
	}
	addon.Status.LastTransitionTime = metav1.Now()

	return r.Status().Patch(ctx, addon, patch)
}
