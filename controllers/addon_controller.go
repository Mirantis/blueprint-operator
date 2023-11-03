package controllers

import (
	"context"
	"io"
	"net/http"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	"github.com/mirantis/boundless-operator/pkg/helm"
	"github.com/mirantis/boundless-operator/pkg/manifest"
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

	if instance.DeletionTimestamp != nil {
		logger.Info("Should remove addon", "Name", req.Name)
		return ctrl.Result{}, nil
	}

	chart := helm.Chart{
		Name:    instance.Spec.Chart.Name,
		Repo:    instance.Spec.Chart.Repo,
		Version: instance.Spec.Chart.Version,
		Set:     instance.Spec.Chart.Set,
		Values:  instance.Spec.Chart.Values,
	}

	logger.Info("Sakshi:::Reconciler... instance details", "Name", instance.Spec.Chart.Name)

	hc := helm.NewHelmChartController(r.Client, logger)
	logger.Info("Creating Addon HelmChart resource", "Name", chart.Name, "Version", chart.Version)
	if err2 := hc.CreateHelmChart(chart); err2 != nil {
		logger.Error(err, "failed to install addon", "Name", chart.Name, "Version", chart.Version)
		return ctrl.Result{Requeue: true}, err2
	}

	// Code for manifest
	var client http.Client
	logger.Info("URL received", "URL", instance.Spec.Manifest.URL)
	resp, err := client.Get(instance.Spec.Manifest.URL)
	if err != nil {
		logger.Error(err, "failed to install addon : Manifest, Unable to read response")
		return ctrl.Result{Requeue: true}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error(err, "failed to install addon : Manifest, unable read bytes")
			return ctrl.Result{Requeue: true}, err
		}

		mc := manifest.NewManifestController(r.Client, logger)
		_, err = mc.Deserialize(bodyBytes)
		if err != nil {
			logger.Error(err, "failed to deserialize manifest")
			return ctrl.Result{Requeue: true}, err
		}

	} else {
		logger.Error(err, "failed to install addon : Manifest, Http status NOT OK")
		return ctrl.Result{Requeue: true}, err
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
