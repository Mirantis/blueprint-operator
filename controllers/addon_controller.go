package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	"github.com/mirantis/boundless-operator/pkg/helm"
)

// AddonReconciler reconciles a Addon object
type AddonReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=addons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=addons/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=addons/finalizers,verbs=update

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

	chart := helm.Chart{
		Name:    instance.Spec.Chart.Name,
		Repo:    instance.Spec.Chart.Repo,
		Version: instance.Spec.Chart.Version,
		Set:     instance.Spec.Chart.Set,
		Values:  instance.Spec.Chart.Values,
	}

	hc := helm.NewHelmChartController(r.Client, logger)

	addonFinalizerName := "boundless.mirantis.com/finalizer"

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, addonFinalizerName) {
			controllerutil.AddFinalizer(instance, addonFinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(instance, addonFinalizerName) {
			// our finalizer is present, so lets delete the helm chart
			if err := hc.DeleteHelmChart(chart, instance.Spec.Namespace); err != nil {
				// if fail to delete the helm chart here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(instance, addonFinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	logger.Info("Creating Addon HelmChart resource", "Name", chart.Name, "Version", chart.Version)
	if err2 := hc.CreateHelmChart(chart, instance.Spec.Namespace); err2 != nil {
		logger.Error(err, "failed to install addon", "Name", chart.Name, "Version", chart.Version)
		return ctrl.Result{Requeue: true}, err2
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
