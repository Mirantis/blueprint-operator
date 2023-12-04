package controllers

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"

	"github.com/go-logr/logr"
	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	"github.com/mirantis/boundless-operator/pkg/event"
	"github.com/mirantis/boundless-operator/pkg/helm"
	"github.com/mirantis/boundless-operator/pkg/manifest"
	batch "k8s.io/api/batch/v1"
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
	addonHelmchartFinalizer = "boundless.mirantis.com/helmchart-finalizer"
	addonManifestFinalizer  = "boundless.mirantis.com/manifest-finalizer"
	addonIndexName          = "helmchartIndex"
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
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
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
			return ctrl.Result{Requeue: true}, err
		}
		r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Chart Addon %s/%s", instance.Spec.Namespace, instance.Name)

		// unfortunately the HelmChart CR doesn't have any useful events or status we can monitor
		// each helmchart object creates a job that runs the helm install - update status from that instead
		jobName := fmt.Sprintf("helm-install-%s", instance.Spec.Chart.Name)
		job := &batch.Job{}
		err = r.Get(ctx, types.NamespacedName{Namespace: instance.Spec.Namespace, Name: jobName}, job)
		if err != nil {
			// might need some time for helmchart CR to create job
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}

		if err := r.updateHelmchartAddonStatus(ctx, logger, req.NamespacedName, job); err != nil {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}

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
				if err := mc.DeleteManifest(boundlessSystemNamespace, instance.Spec.Name, instance.Spec.Manifest.URL); err != nil {
					// if fail to delete the manifest here, return with error
					// so that it can be retried
					r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedDelete, "Failed to Delete Manifest Addon %s/%s : %s", instance.Spec.Namespace, instance.Name, err)
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

		err = mc.CreateManifest(boundlessSystemNamespace, instance.Spec.Name, instance.Spec.Manifest.URL)
		if err != nil {
			logger.Error(err, "failed to install addon via manifest", "URL", instance.Spec.Manifest.URL)
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "Failed to Create Manifest Addon %s/%s : %s", instance.Spec.Namespace, instance.Name, err)
			return ctrl.Result{Requeue: true}, err
		}

		r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Manifest Addon %s/%s", instance.Spec.Namespace, instance.Name)

		m := &boundlessv1alpha1.Manifest{}
		err = r.Get(ctx, types.NamespacedName{Namespace: boundlessSystemNamespace, Name: instance.Spec.Name}, m)
		if err != nil {
			if errors.IsNotFound(err) {
				// might need some time for CR to  be created
				r.updateStatus(ctx, logger, req.NamespacedName, boundlessv1alpha1.TypeComponentProgressing, "Awaiting Manifest Resource Creation")
			}
			return ctrl.Result{}, err
		}

		logger.Info("Set owner ref field on manifest")
		if err := controllerutil.SetControllerReference(instance, m, r.Scheme); err != nil {
			logger.Error(err, "Failed to set owner reference on manifest", "ManifestName", m.Name)
			return ctrl.Result{}, err
		}

		if err := r.Update(ctx, m); err != nil {
			return ctrl.Result{}, err
		}

		if m.Status.Type == "" || m.Status.Reason == "" {
			err = r.updateStatus(ctx, logger, req.NamespacedName, boundlessv1alpha1.TypeComponentProgressing, "Awaiting status from manifest object")
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err

		}
		err = r.updateStatus(ctx, logger, req.NamespacedName, m.Status.Type, m.Status.Reason, m.Status.Message)
		if err != nil {
			return ctrl.Result{}, err
		}

	default:
		logger.Info("Unknown AddOn kind", "Kind", instance.Spec.Kind)
		return ctrl.Result{Requeue: false}, fmt.Errorf("Unknown AddOn Kind: %w", err)
	}

	logger.Info("Finished reconcile request on MkeAddon instance", "Name", req.Name)
	return ctrl.Result{Requeue: false}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// attaches an index onto the Addon
	// index key is addonIndexName
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &boundlessv1alpha1.Addon{}, addonIndexName, func(rawObj client.Object) []string {
		addon := rawObj.(*boundlessv1alpha1.Addon)
		if addon.Spec.Chart != nil && addon.Spec.Chart.Name != "" {

			jobName := fmt.Sprintf("helm-install-%s", addon.Spec.Chart.Name)
			return []string{fmt.Sprintf("%s-%s", addon.Spec.Namespace, jobName)}
		}
		// don't add this index for non helm-chart type addons
		return nil
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Addon{}).
		Owns(&boundlessv1alpha1.Manifest{}).
		Watches(
			&source.Kind{Type: &batch.Job{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForJob),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *AddonReconciler) findObjectsForJob(job client.Object) []reconcile.Request {
	attachedAddonList := &boundlessv1alpha1.AddonList{}
	err := r.List(context.TODO(), attachedAddonList, client.MatchingFields{addonIndexName: fmt.Sprintf("%s-%s", job.GetNamespace(), job.GetName())})
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(attachedAddonList.Items))
	for i, item := range attachedAddonList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

func (r *AddonReconciler) updateHelmchartAddonStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, job *batch.Job) error {
	logger.Info("Updating Helm Chart Addon Status")
	if job.Status.CompletionTime != nil && job.Status.Succeeded > 0 {
		err := r.updateStatus(ctx, logger, namespacedName, boundlessv1alpha1.TypeComponentAvailable, fmt.Sprintf("Helm Chart %s successfully installed", job.Name))
		if err != nil {
			return err
		}
	} else if job.Status.StartTime != nil && job.Status.Failed > 0 {
		err := r.updateStatus(ctx, logger, namespacedName, boundlessv1alpha1.TypeComponentUnhealthy, fmt.Sprintf("Helm Chart %s install has failed", job.Name))
		if err != nil {
			return err
		}
	} else {
		err := r.updateStatus(ctx, logger, namespacedName, boundlessv1alpha1.TypeComponentProgressing, fmt.Sprintf("Helm Chart %s install still progressing", job.Name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *AddonReconciler) updateStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, conditionTypeToApply boundlessv1alpha1.StatusType, reasonToApply string, messageToApply ...string) error {
	logger.Info("Update status with type and reason", "TypeToApply", conditionTypeToApply, "ReasonToApply", reasonToApply)

	addon := &boundlessv1alpha1.Addon{}
	err := r.Get(ctx, namespacedName, addon)
	if err != nil {
		logger.Error(err, "Failed to get addon to update status")
		return err
	}

	nilStatus := boundlessv1alpha1.AddonStatus{}
	if addon.Status != nilStatus && addon.Status.Type == conditionTypeToApply && addon.Status.Reason == reasonToApply {
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
