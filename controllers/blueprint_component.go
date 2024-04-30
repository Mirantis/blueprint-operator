package controllers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
)

// Component represents a blueprint component
type Component interface {
	// GetComponentName return the name of the component
	GetComponentName() string
	// GetComponentNamespace returns the namespace required by the component
	GetComponentNamespace() string
	// GetObject return the kube object representing the component
	GetObject() client.Object
	// SetObject sets the component object to the supplied kube object
	SetObject(obj client.Object) error
}

// ComponentSpec represents the specs of the component
type ComponentSpec[C Component] interface {
	// GetName returns the desired name of the component
	GetName() string
	// GetNamespace returns the desired namespace of the component
	GetNamespace() string
	// SetNamespace sets the namespace of the component
	SetNamespace(namespace string)
	// IsClusterScoped returns true if the component is cluster scoped, cluster scoped components don't have namespaces
	IsClusterScoped() bool
	// MakeComponent creates a new component object from the spec
	MakeComponent() C
}

type ComponentList[T Component] interface {
	GetItems() []T
	GetObjectList() client.ObjectList
}

func listInstalledComponents[C Component](ctx context.Context, logger logr.Logger, apiClient client.Client, list ComponentList[C]) (map[string]C, error) {
	if err := apiClient.List(ctx, list.GetObjectList()); err != nil {
		return nil, err
	}

	logger.Info("existing items are", "names", list)
	itemsToUninstall := make(map[string]C)

	for _, item := range list.GetItems() {
		itemsToUninstall[item.GetComponentName()] = item
	}

	return itemsToUninstall, nil
}

func deleteComponents[C Component](ctx context.Context, logger logr.Logger, apiClient client.Client, componentsToUninstall map[string]C) error {
	for _, component := range componentsToUninstall {
		logger.Info("Removing object", "Name", component.GetComponentName(), "Namespace", component.GetComponentNamespace())
		if err := apiClient.Delete(ctx, component.GetObject(), client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to remove object", "Name", component.GetComponentName())
			return err
		}
	}

	return nil
}

// createEmptyCopy creates an empty copy of the desired component, the component must be a pointer
func createEmptyCopy[C Component](desired C) (C, error) {
	cType := reflect.TypeOf(desired)
	if cType.Kind() != reflect.Ptr {
		return *new(C), fmt.Errorf("component must be a pointer, got %s", cType.Kind().String())
	}
	return reflect.New(cType.Elem()).Interface().(C), nil
}

func createOrUpdateComponent[C Component](ctx context.Context, logger logr.Logger, apiClient client.Client, desired C) error {
	desiredObj := desired.GetObject()

	existing, err := createEmptyCopy(desired)
	if err != nil {
		return err
	}

	existingObj := existing.GetObject()
	err = apiClient.Get(ctx, client.ObjectKey{Name: desiredObj.GetName(), Namespace: desiredObj.GetNamespace()}, existingObj)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if err = existing.SetObject(existingObj); err != nil {
		return fmt.Errorf("failed to set existing object to the component: %w", err)
	}

	if existingObj.GetName() != "" {
		logger.Info("Component already exists. Updating", "Name", existingObj.GetName(), "Spec.Namespace", existing.GetComponentNamespace())

		if desired.GetComponentNamespace() == existing.GetComponentNamespace() {
			desiredObj.SetResourceVersion(existingObj.GetResourceVersion())
			// TODO : Copy all the fields from the existing
			desiredObj.SetFinalizers(existingObj.GetFinalizers())
			err = apiClient.Update(ctx, desiredObj)
			if err != nil {
				return fmt.Errorf("failed to update component %s: %w", existingObj.GetName(), err)
			}

			return nil
		} else {
			// the component has moved namespaces, we need to delete and re-create it
			logger.Info("Component has moved namespaces, deleting old version of the component",
				"Name", desired.GetComponentName(),
				"Old Namespace", existing.GetComponentNamespace(),
				"New Namespace", desired.GetComponentNamespace())
			if err = apiClient.Delete(ctx, existingObj, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				logger.Error(err, "Failed to remove old version of component", "Name", existingObj.GetName())
				return err
			}
		}
	}

	logger.Info("Creating component", "Name", desired.GetComponentName(), "Spec.Namespace", desired.GetComponentNamespace())
	err = apiClient.Create(ctx, desiredObj)
	if err != nil {
		return fmt.Errorf("failed to create component %s: %w", desired.GetComponentName(), err)
	}

	return nil
}

func reconcileComponents[C Component, S ComponentSpec[C]](ctx context.Context, logger logr.Logger, apiClient client.Client,
	instance *boundlessv1alpha1.Blueprint, specs []S, listObject ComponentList[C]) error {

	componentsToUninstall, err := listInstalledComponents(ctx, logger, apiClient, listObject)
	if err != nil {
		return err
	}

	for _, spec := range specs {
		if !spec.IsClusterScoped() {
			if spec.GetNamespace() == "" {
				spec.SetNamespace(instance.Namespace)
			}

			err = utils.CreateNamespaceIfNotExist(apiClient, ctx, logger, spec.GetNamespace())
			if err != nil {
				return fmt.Errorf("unable to create component namespace: %w", err)
			}
		}

		logger.Info("Reconciling component", "Name", spec.GetName(), "Spec.Namespace", spec.GetNamespace())
		component := spec.MakeComponent()
		err = createOrUpdateComponent(ctx, logger, apiClient, component)
		if err != nil {
			logger.Error(err, "Failed to reconcile component", "Name", spec.GetName(), "Spec.Namespace", spec.GetNamespace())
			return err
		}

		// if the component is in the spec, we shouldn't uninstall it
		delete(componentsToUninstall, component.GetComponentName())
	}

	if len(componentsToUninstall) > 0 {
		err = deleteComponents(ctx, logger, apiClient, componentsToUninstall)
		if err != nil {
			return err
		}
	}

	return nil
}
