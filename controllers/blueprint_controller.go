package controllers

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/blueprint"
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

	err := reconcileComponents[*boundlessv1alpha1.Addon, *boundlessv1alpha1.AddonSpec](ctx, logger, r.Client, instance,
		utils.PointSlice(instance.Spec.Components.Addons), &boundlessv1alpha1.AddonList{})
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconcileComponents[*blueprint.Issuer, *boundlessv1alpha1.Issuer](ctx, logger, r.Client, instance,
		utils.PointSlice(instance.Spec.CAs.Issuers), &blueprint.IssuerList{})
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconcileComponents[*blueprint.ClusterIssuer, *boundlessv1alpha1.ClusterIssuer](ctx, logger, r.Client, instance,
		utils.PointSlice(instance.Spec.CAs.ClusterIssuers), &blueprint.ClusterIssuerList{})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlueprintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Blueprint{}).
		Complete(r)
}
