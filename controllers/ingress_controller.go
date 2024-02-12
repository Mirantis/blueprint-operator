package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/helm"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=ingresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=ingresses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Ingress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err error
		//requeueRequest bool
	)

	_ = log.FromContext(ctx)

	logger := log.FromContext(ctx)
	logger.Info("Reconcile request on Ingress instance", "Name", req.Name)

	instance := &boundlessv1alpha1.Ingress{}
	err = r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		msg := "failed to get MkeIngress instance"
		if errors.IsNotFound(err) {
			// Ignore request.
			logger.Info(msg, "Name", req.Name, "Requeue", false)
			return ctrl.Result{}, nil
		}
		logger.Error(err, msg, "Name", req.Namespace, "Requeue", true)
		return ctrl.Result{}, err
	}
	// Perform necessary cleanup.
	// If the cleanup succeeds, remove the finalizer.
	// If the cleanup fails, requeue the request.
	if instance.DeletionTimestamp != nil {
		if instance.Status.IngressReady {
			//_, err := r.ManageIngress(ctx, instance.Spec, "delete")
			//if err != nil {
			//	logger.Error(err, "failed to remove ingress")
			//	return reconcile.Result{}, err
			//}
			logger.Info("Should remove ingress", "Name", req.Name)
		}

		//for index, finalizer := range instance.Finalizers {
		//	if finalizer == MkeNgIngressFinalizer {
		//		instance.Finalizers = append(instance.Finalizers[:index], instance.Finalizers[index+1:]...)
		//		logger.Info("Removing mkengingress.mirantis.com finalizer from MkeNgIngress instance", "Name", req.Name)
		//		err = r.Client.Update(ctx, instance)
		//		if err != nil {
		//			logger.Error(err, "failed to update MkeNgIngress instance", "Name", req.Name)
		//		}
		//		return reconcile.Result{}, err
		//	}
		//}
		return ctrl.Result{}, nil
	}

	// Deletion timestamp is not set. Instance was either created or updated on the supervisor API server.
	// Check if finalizer exists. If it doesn't, add finalizer and requeue the object.
	//isFinalizerSet := false
	//for _, finalizer := range instance.Finalizers {
	//	if finalizer == MkeIngressFinalizer {
	//		isFinalizerSet = true
	//		break
	//	}
	//}
	//if !isFinalizerSet {
	//	instance.Finalizers = append(instance.Finalizers, MkeIngressFinalizer)
	//	logger.Info("Adding mkeingress.mirantis.com finalizer to MkeIngress instance", "Name", req.Name)
	//	err = r.Client.Update(ctx, instance)
	//	if err != nil {
	//		logger.Error(err, "failed to update MkeIngress instance", "Name", req.Name)
	//	}
	//	return ctrl.Result{}, err
	//}

	chart := helm.Chart{}
	switch instance.Spec.Provider {
	case "ingress-nginx":
		chart = NginxIngressHelmChart
		break
	case "kong":
		chart = KongIngressHelmChart
		break
	}

	chart.Values = instance.Spec.Config

	hc := helm.NewHelmChartController(r.Client, logger)
	logger.Info("Creating HelmChart resource", "Name", chart.Name, "Version", chart.Version)
	if err2 := hc.CreateHelmChart(chart, instance.Namespace); err2 != nil {
		logger.Error(err, "failed to install ingress controller", "Controller Type", instance.Spec.Provider, "Version", "v1alpha1")
		return ctrl.Result{}, err2
	}

	logger.Info("Setting IngressReady", "Name", instance.Name)
	instance.Status.IngressReady = true
	err = r.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "failed to update MkeIngress status")
	}

	logger.Info("Finished reconcile request on MkeIngress instance", "Name", req.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Ingress{}).
		Complete(r)
}
