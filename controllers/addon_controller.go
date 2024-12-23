package controllers

import (
	"context"
	"fmt"
	"slices"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/pkg/controllers/helm"
	"github.com/mirantiscontainers/blueprint-operator/pkg/controllers/manifest"
	"github.com/mirantiscontainers/blueprint-operator/pkg/event"
	k8s "github.com/mirantiscontainers/blueprint-operator/pkg/kubernetes"
)

const (
	kindManifest = "manifest"
	kindChart    = "chart"
	finalizer    = "blueprint.mirantis.com/addon-finalizer"
)

// AddonReconciler reconciles a Addon object
type AddonReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	helmController     *helm.Controller
	manifestController *manifest.Controller

	SetupLogger logr.Logger
}

//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=addons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=addons/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=addons/finalizers,verbs=update
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=manifests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=manifests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the Addon object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *AddonReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile request on Addon instance", "Name", req.Name)
	start := time.Now()
	var err error
	defer func() {
		AddOnHistVec.WithLabelValues(req.Name, getMetricStatus(err)).Observe(time.Since(start).Seconds())
	}()

	// Initialize k8s client
	k8sClient := k8s.NewClient(logger, r.Client)
	r.helmController = helm.NewHelmChartController(r.Client, k8sClient, logger)
	r.manifestController = manifest.NewManifestController(r.Client, logger)

	instance := &v1alpha1.Addon{}
	if err = r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Addon instance not found. Ignoring since object must be deleted.", "Name", req.Name)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get addon instance", "Name", req.Name, "Requeue", true)
		return ctrl.Result{}, err
	}

	kind := instance.Spec.Kind
	if !slices.Contains([]string{kindChart, kindManifest}, kind) {
		logger.Error(fmt.Errorf("invalid addon kind: %s", instance.Spec.Kind), "Invalid Addon Kind", "Addon", instance)
		// no need to requeue, as we can't do anything with an invalid addon kind
		return ctrl.Result{Requeue: false}, nil
	}

	// validate
	switch kind {
	case kindChart:
		if instance.Spec.Chart == nil {
			logger.Error(fmt.Errorf("invalid addon"), "Chart info is missing", "Addon", instance)
			// no need to requeue, as we can't do anything with an invalid addon specs
			return ctrl.Result{Requeue: false}, nil
		}
	case kindManifest:
		if instance.Spec.Manifest == nil {
			logger.Error(fmt.Errorf("invalid addon"), "Manifest info is missing", "Addon", instance)
			// no need to requeue, as we can't do anything with an invalid addon specs
			return ctrl.Result{Requeue: false}, nil
		}
	}

	// add/remove finalizer
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, finalizer) {
			controllerutil.AddFinalizer(instance, finalizer)
			if err = r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(instance, finalizer) {
			if err = r.deleteAddon(ctx, instance); err != nil {
				// if fail to delete the addon here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(instance, finalizer)
			if err = r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// create or update the addon
	switch kind {
	case kindChart:
		chart := instance.Spec.Chart
		logger.Info("Creating Addon HelmChart resource", "Name", chart.Name, "Version", chart.Version)
		if err = r.helmController.CreateHelmRelease(ctx, instance, instance.Spec.Namespace, instance.Spec.DryRun); err != nil {
			logger.Error(err, "failed to install addon", "Name", chart.Name, "Version", chart.Version)
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "Failed to Create Chart Addon %s/%s : %s", instance.Spec.Namespace, instance.Name, err)
			return ctrl.Result{}, err
		}

		releaseName := instance.Spec.Name
		releaseKey := types.NamespacedName{Namespace: consts.NamespaceBlueprintSystem, Name: releaseName}

		release := &helmv2.HelmRelease{}
		if err = r.Get(ctx, releaseKey, release); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("HelmRelease not yet found", "Name", releaseName, "Requeue", true)
				return ctrl.Result{RequeueAfter: DefaultRequeueDuration}, nil
			}
			return ctrl.Result{}, err
		}

		if err = r.updateHelmChartAddonStatus(ctx, logger, req.NamespacedName, release, instance); err != nil {
			logger.Error(err, "Failed to update Helm Chart Addon status", "Name", releaseName)
			return ctrl.Result{}, err
		}

	case kindManifest:
		if err = r.manifestController.CreateManifest(ctx, consts.NamespaceBlueprintSystem, instance.Spec.Name, instance.Spec.Manifest); err != nil {
			logger.Error(err, "failed to install addon via manifest", "URL", instance.Spec.Manifest.URL)
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "Failed to Create Manifest Addon %s/%s : %s", instance.Spec.Namespace, instance.Name, err)
			return ctrl.Result{}, err
		}

		m := &v1alpha1.Manifest{}
		if err = r.Get(ctx, types.NamespacedName{Namespace: consts.NamespaceBlueprintSystem, Name: instance.Spec.Name}, m); err != nil {
			if apierrors.IsNotFound(err) {
				// might need some time for CR to  be created
				r.updateStatus(ctx, logger, req.NamespacedName, v1alpha1.TypeComponentProgressing, "Awaiting Manifest Resource Creation")
				logger.Info("Manifest resources not yet found", "Name", instance.Spec.Name, "Requeue", true)
				return ctrl.Result{RequeueAfter: DefaultRequeueDuration}, nil
			}
			logger.Error(err, "Failed to get manifest resource", "Name", instance.Spec.Name)
			return ctrl.Result{}, err
		}

		if err = r.setOwnerReferenceOnManifest(ctx, logger, instance, m); err != nil {
			return ctrl.Result{}, err
		}

		if err = r.updateManifestAddonStatus(ctx, logger, instance, m); err != nil {
			return ctrl.Result{}, err
		}

	}

	logger.Info("Finished reconcile request on Addon instance", "Name", req.Name)
	return ctrl.Result{}, nil
}

func (r *AddonReconciler) deleteAddon(ctx context.Context, addon *v1alpha1.Addon) error {
	switch addon.Spec.Kind {
	case kindChart:
		if err := r.helmController.DeleteHelmRelease(ctx, addon); err != nil {
			r.Recorder.AnnotatedEventf(addon, map[string]string{event.AddonAnnotationKey: addon.Name}, event.TypeWarning, event.ReasonFailedDelete, "Failed to Delete Chart Addon %s/%s: %s", addon.Spec.Namespace, addon.Name, err)
			return err
		}
	case kindManifest:
		// our finalizer is present, so lets delete the helm chart
		if err := r.manifestController.DeleteManifest(ctx, consts.NamespaceBlueprintSystem, addon.Spec.Name, addon.Spec.Manifest.URL); err != nil {
			r.Recorder.AnnotatedEventf(addon, map[string]string{event.AddonAnnotationKey: addon.Name}, event.TypeWarning, event.ReasonFailedDelete, "Failed to Delete Manifest Addon %s/%s : %s", addon.Spec.Namespace, addon.Name, err)
			return err
		}
	default:
		return fmt.Errorf("invalid addon kind: %s", addon.Spec.Kind)
	}
	return nil
}

// updateManifestAddonStatus checks if the manifest associated with the addon has a status to bubble up to addon and updates addon if so
func (r *AddonReconciler) updateManifestAddonStatus(ctx context.Context, logger logr.Logger, addon *v1alpha1.Addon, manifest *v1alpha1.Manifest) error {
	if manifest.Status.Type == "" || manifest.Status.Reason == "" {
		err := r.updateStatus(ctx, logger, types.NamespacedName{Namespace: addon.Namespace, Name: addon.Name}, v1alpha1.TypeComponentProgressing, "Awaiting status from manifest object")
		if err != nil {
			return err
		}
		// manifest has no status yet so nothing to do
		return nil
	}

	if manifest.Status.Type == v1alpha1.TypeComponentAvailable && addon.Status.Type != v1alpha1.TypeComponentAvailable {
		// we are about to update the addon status from not available to available so let's emit an event
		r.Recorder.AnnotatedEventf(addon, map[string]string{event.AddonAnnotationKey: addon.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Manifest Addon %s/%s", addon.Spec.Namespace, addon.Name)
	}

	err := r.updateStatus(ctx, logger, types.NamespacedName{Namespace: addon.Namespace, Name: addon.Name}, manifest.Status.Type, manifest.Status.Reason, manifest.Status.Message)
	if err != nil {
		return err
	}
	return nil
}

// setOwnerReferenceOnManifest sets the owner reference on the manifest object to point to the addon object
// This effectively causes the owner addon to be reconciled when the manifest is updated.
func (r *AddonReconciler) setOwnerReferenceOnManifest(ctx context.Context, logger logr.Logger, addon *v1alpha1.Addon, manifest *v1alpha1.Manifest) error {
	logger.Info("Set owner ref field on manifest")
	if err := controllerutil.SetControllerReference(addon, manifest, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference on manifest", "ManifestName", manifest.Name)
		return err
	}

	if err := r.Update(ctx, manifest); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Addon{}).
		Owns(&v1alpha1.Manifest{}).
		Owns(&helmv2.HelmRelease{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}

// updateHelmChartAddonStatus checks the status of the associated helm release and updates the status of the Addon CR accordingly
func (r *AddonReconciler) updateHelmChartAddonStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, release *helmv2.HelmRelease, addon *v1alpha1.Addon) error {
	logger.Info("Updating Helm Chart Addon Status")
	releaseStatus := helm.DetermineReleaseStatus(release)
	if releaseStatus == helm.ReleaseStatusSuccess {
		r.Recorder.AnnotatedEventf(addon, map[string]string{event.AddonAnnotationKey: addon.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Chart Addon %s/%s", addon.Spec.Namespace, addon.Name)
		err := r.updateStatus(ctx, logger, namespacedName, v1alpha1.TypeComponentAvailable, fmt.Sprintf("Helm Chart %s successfully installed", release.Name))
		if err != nil {
			return err
		}
	} else if releaseStatus == helm.ReleaseStatusFailed {
		r.Recorder.AnnotatedEventf(addon, map[string]string{event.AddonAnnotationKey: addon.Name}, event.TypeWarning, event.ReasonFailedCreate, "Helm Chart Addon %s/%s has failed to install", addon.Spec.Namespace, addon.Name)
		err := r.updateStatus(ctx, logger, namespacedName, v1alpha1.TypeComponentUnhealthy, fmt.Sprintf("Helm Chart %s install has failed", release.Name))
		if err != nil {
			return err
		}
	} else {
		err := r.updateStatus(ctx, logger, namespacedName, v1alpha1.TypeComponentProgressing, fmt.Sprintf("Helm Chart %s install still progressing", release.Name))
		if err != nil {
			return err
		}
	}
	return nil
}

// updateStatus queries for a fresh Addon with the provided namespacedName.
// This avoids some errors where we fail to update status because we have an older (stale) version of the object
// It then updates the Addon's status fields with the provided type, reason, and optionally message.
func (r *AddonReconciler) updateStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, typeToApply v1alpha1.StatusType, reasonToApply string, messageToApply ...string) error {
	logger.Info("Update status with type and reason", "TypeToApply", typeToApply, "ReasonToApply", reasonToApply)

	addon := &v1alpha1.Addon{}
	err := r.Get(ctx, namespacedName, addon)
	if err != nil {
		logger.Error(err, "Failed to get addon to update status")
		return err
	}

	nilStatus := v1alpha1.AddonStatus{}
	if addon.Status != nilStatus && addon.Status.Type == typeToApply && addon.Status.Reason == reasonToApply {
		// avoid infinite reconciliation loops
		logger.Info("No updates to status needed")
		return nil
	}

	logger.Info("Update status for addon", "Name", addon.Name)

	patch := client.MergeFrom(addon.DeepCopy())
	addon.Status.Type = typeToApply
	addon.Status.Reason = reasonToApply
	if len(messageToApply) > 0 {
		addon.Status.Message = messageToApply[0]
	}
	addon.Status.LastTransitionTime = metav1.Now()

	return r.Status().Patch(ctx, addon, patch)
}
