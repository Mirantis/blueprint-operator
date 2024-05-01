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

// AbstractObject is a wrapper for client.Object that makes it more reusable
type AbstractObject interface {
	// GetObjectName return the name of the object
	GetObjectName() string
	// GetObjectNamespace returns the namespace required by the object
	GetObjectNamespace() string
	// GetObject return the client.Object representation of the AbstractObject
	GetObject() client.Object
	// SetObject updates the client.Object managed by the AbstractObject
	SetObject(obj client.Object) error
}

// AbstractObjectSpec represents the specs of the object
type AbstractObjectSpec[O AbstractObject] interface {
	// GetName returns the desired name of the object
	GetName() string
	// GetNamespace returns the desired namespace of the object
	GetNamespace() string
	// SetNamespace sets the namespace of the object
	SetNamespace(namespace string)
	// IsClusterScoped returns true if the object is cluster scoped, cluster scoped object don't have namespaces
	IsClusterScoped() bool
	// MakeObject creates a new object from the spec
	MakeObject() O
}

type AbstractObjectList[O AbstractObject] interface {
	GetItems() []O
	GetObjectList() client.ObjectList
}

func listInstalledObjects[O AbstractObject](ctx context.Context, logger logr.Logger, apiClient client.Client, list AbstractObjectList[O]) (map[string]O, error) {
	if err := apiClient.List(ctx, list.GetObjectList()); err != nil {
		return nil, err
	}

	logger.Info("existing items are", "names", list)
	itemsToUninstall := make(map[string]O)

	for _, item := range list.GetItems() {
		itemsToUninstall[item.GetObjectName()] = item
	}

	return itemsToUninstall, nil
}

func deleteObjects[O AbstractObject](ctx context.Context, logger logr.Logger, apiClient client.Client, objectsToUninstall map[string]O) error {
	for _, o := range objectsToUninstall {
		logger.Info("Removing object", "Name", o.GetObjectName(), "Namespace", o.GetObjectNamespace())
		if err := apiClient.Delete(ctx, o.GetObject(), client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to remove object", "Name", o.GetObjectName())
			return err
		}
	}

	return nil
}

// createEmptyCopy creates an empty copy of the desired object, the object must be a pointer
func createEmptyCopy[O AbstractObject](desired O) (O, error) {
	cType := reflect.TypeOf(desired)
	if cType.Kind() != reflect.Ptr {
		return *new(O), fmt.Errorf("object must be a pointer, got %s", cType.Kind().String())
	}
	return reflect.New(cType.Elem()).Interface().(O), nil
}

func createOrUpdateObject[O AbstractObject](ctx context.Context, logger logr.Logger, apiClient client.Client, desired O) error {
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
		return fmt.Errorf("failed to set existing object to the abstract object: %w", err)
	}

	if existingObj.GetName() != "" {
		logger.Info("Object already exists. Updating", "Name", existingObj.GetName(), "Spec.Namespace", existing.GetObjectNamespace())

		if desired.GetObjectNamespace() == existing.GetObjectNamespace() {
			desiredObj.SetResourceVersion(existingObj.GetResourceVersion())
			// TODO : Copy all the fields from the existing
			desiredObj.SetFinalizers(existingObj.GetFinalizers())
			err = apiClient.Update(ctx, desiredObj)
			if err != nil {
				return fmt.Errorf("failed to update object %s: %w", existingObj.GetName(), err)
			}

			return nil
		} else {
			// the object has moved namespaces, we need to delete and re-create it
			logger.Info("Object has moved namespaces, deleting old version of the object",
				"Name", desired.GetObjectName(),
				"Old Namespace", existing.GetObjectNamespace(),
				"New Namespace", desired.GetObjectNamespace())
			if err = apiClient.Delete(ctx, existingObj, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				logger.Error(err, "Failed to remove old version of object", "Name", existingObj.GetName())
				return err
			}
		}
	}

	logger.Info("Creating object", "Name", desired.GetObjectName(), "Spec.Namespace", desired.GetObjectNamespace())
	err = apiClient.Create(ctx, desiredObj)
	if err != nil {
		return fmt.Errorf("failed to create object %s: %w", desired.GetObjectName(), err)
	}

	return nil
}

func reconcileObjects[C AbstractObject, S AbstractObjectSpec[C]](ctx context.Context, logger logr.Logger, apiClient client.Client,
	instance *boundlessv1alpha1.Blueprint, specs []S, listObject AbstractObjectList[C]) error {

	objectsToUninstall, err := listInstalledObjects(ctx, logger, apiClient, listObject)
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
				return fmt.Errorf("unable to create object namespace: %w", err)
			}
		}

		logger.Info("Reconciling object", "Name", spec.GetName(), "Spec.Namespace", spec.GetNamespace())
		object := spec.MakeObject()
		err = createOrUpdateObject(ctx, logger, apiClient, object)
		if err != nil {
			logger.Error(err, "Failed to reconcile object", "Name", spec.GetName(), "Spec.Namespace", spec.GetNamespace())
			return err
		}

		// if the object is in the spec, we shouldn't uninstall it
		delete(objectsToUninstall, object.GetObjectName())
	}

	if len(objectsToUninstall) > 0 {
		err = deleteObjects(ctx, logger, apiClient, objectsToUninstall)
		if err != nil {
			return err
		}
	}

	return nil
}
