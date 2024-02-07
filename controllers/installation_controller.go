package controllers

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operator "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/controllers/installation"
)

var (
	DefaultInstanceKey = client.ObjectKey{Name: "default"}
)

// InstallationReconciler reconciles a Installation object
type InstallationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=installations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=installations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=installations/finalizers,verbs=update

// Reconcile reconciles the Installation resource and installs the necessary components
// such as helm controller and cert manager.
func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	logger.Info("Reconciling Installation instance")

	// Get the installation object if it exists so that we can save the original
	// status before we merge/fill that object with other values.
	instance := &operator.Installation{}
	if err := r.Client.Get(ctx, DefaultInstanceKey, instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Installation config not found")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "An error occurred when querying the Installation resource")
		return reconcile.Result{}, err
	}

	// Install helm controller if it does not exist
	exists, err := installation.CheckHelmControllerExists(ctx, r.Client)
	if err != nil {
		logger.Error(err, "failed to check if helm controller already exists")
		return ctrl.Result{}, fmt.Errorf("failed to check if helm controller already exists")
	}
	if !exists {
		logger.Info("Helm controller is not installed. Installing...")
		if err = installation.InstallHelmController(ctx, r.Client, logger); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Install cert manager if it does not exist
	exist, err := installation.CheckIfCertManagerAlreadyExists(ctx, r.Client, logger)
	if err != nil {
		logger.Error(err, "failed to check if cert manager already exists")
		return ctrl.Result{}, fmt.Errorf("failed to check if cert manager already exists")
	}
	if !exist {
		logger.Info("cert manager is not installed. Installing...")
		if err = installation.InstallCertManager(ctx, r.Client, logger); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		logger.Info("cert manager is already installed.")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operator.Installation{}).
		Complete(r)
}
