/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	"github.com/mirantis/boundless-operator/pkg/controllers/installation"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=clusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Cluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	logger := log.FromContext(ctx)
	logger.Info("Reconcile request on Cluster instance", "Name", req.Name)
	instance := &boundlessv1alpha1.Cluster{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		logger.Error(err, "Failed to get Cluster instance")
		return ctrl.Result{}, err
	}

	exists, err := installation.CheckHelmControllerExists(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exists {
		logger.Info("Helm controller is not installed")
		logger.Info("Installing helm controller")
		err := installation.InstallHelmController(ctx, r.Client, logger)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	logger.Info("Reconciling ingress")
	err = r.createOrUpdateIngress(ctx, logger, ingressResource(&instance.Spec.Components.Core.Ingress))
	if err != nil {
		logger.Error(err, "Failed to reconcile ingress", "Name", instance.Spec.Components.Core.Ingress)
		return ctrl.Result{Requeue: true}, err
	}

	for _, addon := range instance.Spec.Components.Addons {
		if addon.Namespace == "" {
			addon.Namespace = instance.Namespace
		}

		logger.Info("Reconciling addon", "Name", addon.Name)
		err = r.createOrUpdateAddon(ctx, logger, addonResource(&addon))
		if err != nil {
			logger.Error(err, "Failed to reconcile addon", "Name", addon.Name)
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) createOrUpdateAddon(ctx context.Context, logger logr.Logger, obj client.Object) error {
	existing := &boundlessv1alpha1.Addon{}
	err := r.Get(ctx, client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if existing.Name != "" {
		logger.Info("Add-on already exists. Updating", "Name", existing.Name)
		obj.SetResourceVersion(existing.GetResourceVersion())
		err = r.Update(ctx, obj)
		if err != nil {
			return fmt.Errorf("failed to update add-on %s: %w", existing.Name, err)
		}
		return nil
	}

	logger.Info("Creating add-on", "Name", existing.Name)
	err = r.Create(ctx, obj)
	if err != nil {
		return fmt.Errorf("failed to create add-on %s: %w", obj.GetName(), err)
	}
	return nil
}

func (r *ClusterReconciler) createOrUpdateIngress(ctx context.Context, logger logr.Logger, obj client.Object) error {
	existing := &boundlessv1alpha1.Ingress{}
	err := r.Get(ctx, client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if existing.Name != "" {
		logger.Info("Ingress already exists. Updating", "Name", existing.Name)
		obj.SetResourceVersion(existing.GetResourceVersion())
		err = r.Update(ctx, obj)
		if err != nil {
			return fmt.Errorf("failed to update ingress %s: %w", existing.Name, err)
		}
		return nil
	}

	logger.Info("Creating ingress", "Name", existing.Name)
	err = r.Create(ctx, obj)
	if err != nil {
		return fmt.Errorf("failed to create ingress %s: %w", obj.GetName(), err)
	}
	return nil
}

func ingressResource(spec *boundlessv1alpha1.IngressSpec) *boundlessv1alpha1.Ingress {
	name := fmt.Sprintf("mke-%s", spec.Provider)
	return &boundlessv1alpha1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: v1.NamespaceDefault,
		},
		Spec: boundlessv1alpha1.IngressSpec{
			Enabled:  spec.Enabled,
			Provider: spec.Provider,
			Config:   spec.Config,
		},
	}
}

func addonResource(spec *boundlessv1alpha1.AddonSpec) *boundlessv1alpha1.Addon {
	name := fmt.Sprintf("mke-%s", spec.Chart.Name)

	return &boundlessv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: v1.NamespaceDefault,
		},
		Spec: boundlessv1alpha1.AddonSpec{
			Chart: boundlessv1alpha1.Chart{
				Name:    spec.Chart.Name,
				Repo:    spec.Chart.Repo,
				Version: spec.Chart.Version,
				Set:     spec.Chart.Set,
				Values:  spec.Chart.Values,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Cluster{}).
		Complete(r)
}
