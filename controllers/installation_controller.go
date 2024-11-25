package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/components"
	"github.com/mirantiscontainers/blueprint-operator/pkg/components/certmanager"
	"github.com/mirantiscontainers/blueprint-operator/pkg/components/fluxcd"
	"github.com/mirantiscontainers/blueprint-operator/pkg/components/webhook"
)

var (
	DefaultInstanceKey = client.ObjectKey{Name: "default", Namespace: "default"}
)

// InstallationReconciler reconciles a Installation object
type InstallationReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	SetupLogger logr.Logger
}

var installationFinalizer = "blueprint.mirantis.com/installation-finalizer"

//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=installations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=installations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=installations/finalizers,verbs=update

// Reconcile reconciles the Installation resource and installs the necessary components
// such as helm controller and cert manager.
func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Installation instance")
	start := time.Now()
	// Get the installation object if it exists so that we can save the original
	// status before we merge/fill that object with other values.
	instance := &v1alpha1.Installation{}
	if err := r.Client.Get(ctx, DefaultInstanceKey, instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Installation instance not found")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "An error occurred when querying the Installation resource", "Name", req.Name)
		return reconcile.Result{}, err
	}

	// list of components to install
	componentList := []components.Component{
		fluxcd.NewFluxCDComponent(r.Client, logger),
		certmanager.NewCertManagerComponent(r.Client, logger),
		webhook.NewWebhookComponent(r.Client, logger),
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(instance, installationFinalizer) {
			logger.Info("Adding Finalizer for Installation")
			controllerutil.AddFinalizer(instance, installationFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				logger.Error(err, "Failed to update Installation resource to add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		logger.Info("Uninstalling components")
		for _, component := range componentList {
			if err := component.Uninstall(ctx); err != nil {
				logger.Error(err, "Failed to uninstall component", "Name", component.Name())
				InstallationHistVec.WithLabelValues(component.Name(), metricsOperationUninstall, metricStatusFailure).Observe(time.Since(start).Seconds())
				return ctrl.Result{}, err
			}
			InstallationHistVec.WithLabelValues(component.Name(), metricsOperationUninstall, metricStatusSuccess).Observe(time.Since(start).Seconds())
		}

		// remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(instance, installationFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Install components
	for _, component := range componentList {
		exists, err := component.CheckExists(ctx)
		if err != nil {
			logger.Error(err, "failed to check if component already exists", "Name", component.Name())
			return ctrl.Result{}, err
		}

		if !exists {
			logger.Info("Component is not installed. Installing...", "Name", component.Name())
			if err = component.Install(ctx); err != nil {
				InstallationHistVec.WithLabelValues(component.Name(), metricsOperationInstall, metricStatusFailure).Observe(time.Since(start).Seconds())
				return ctrl.Result{}, err
			}
			InstallationHistVec.WithLabelValues(component.Name(), metricsOperationInstall, metricStatusSuccess).Observe(time.Since(start).Seconds())
		} else {
			logger.Info("Component is already installed", "Name", component.Name())
		}
	}
	logger.V(1).Info("Finished reconciling Installation")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Installation{}).Complete(r); err != nil {
		return err
	}

	// try to create installation object
	TryCreateInstallationResource(r.SetupLogger, r.Client)
	return nil
}

// TryCreateInstallationResource creates the Installation resource if it does not exist
// If the resource already exists, or if an error occurs, it logs the error and returns
// without taking any action.
func TryCreateInstallationResource(log logr.Logger, client client.Client) {
	obj := &v1alpha1.Installation{ObjectMeta: metav1.ObjectMeta{Name: DefaultInstanceKey.Name, Namespace: DefaultInstanceKey.Namespace}}
	if err := client.Create(context.Background(), obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("Installation resource already exists")
			return
		}
		log.Error(err, "Installation resource has failed to create, blueprint operator may not function properly. "+
			"Please create the Installation resource manually.")
		return
	}
}
