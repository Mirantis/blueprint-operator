package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
)

type ItemsLister func(ctx context.Context, apiClient client.Client) ([]client.Object, error)

func generateName(obj client.Object) string {
	if obj.GetNamespace() == "" {
		return obj.GetName()
	}
	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
}

func listInstalledObjects(ctx context.Context, logger logr.Logger, apiClient client.Client,
	lister ItemsLister) (map[string]client.Object, error) {

	items, err := lister(ctx, apiClient)
	if err != nil {
		return nil, err
	}

	installedItems := make(map[string]client.Object)
	for _, item := range items {
		if item.GetLabels()["app.kubernetes.io/part-of"] == "blueprint-operator" {
			logger.V(4).Info("skipping BOP object", "Name", item.GetName(), "Namespace", item.GetNamespace())
			continue
		}

		installedItems[generateName(item)] = item
	}

	logger.V(4).Info("installed items", "names", installedItems)

	return installedItems, nil
}

func deleteObjects(ctx context.Context, logger logr.Logger, apiClient client.Client, objectsToUninstall map[string]client.Object) error {
	for _, o := range objectsToUninstall {
		// Only delete the resources(cert/issuer) that are managed by BOP.
		// This check can be removed once we add the label in all
		// the objects created by BOP (https://mirantis.jira.com/browse/BOP-919).
		if o.GetObjectKind().GroupVersionKind().Kind == "Certificate" || o.GetObjectKind().GroupVersionKind().Kind == "Issuer" {
			if o.GetLabels()["app.kubernetes.io/managed-by"] != "blueprint-operator" {
				logger.Info("Skipping deletion of ", "Kind", o.GetObjectKind().GroupVersionKind().Kind)
				continue
			}
		}

		logger.Info("Removing object", "Name", o.GetName(), "Namespace", o.GetNamespace())
		if err := apiClient.Delete(ctx, o, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to remove object", "Name", o.GetName())
			return err
		}
	}

	return nil
}

func createOrUpdateObject(ctx context.Context, logger logr.Logger, apiClient client.Client, desired client.Object) error {
	existing := desired.DeepCopyObject().(client.Object)

	err := apiClient.Get(ctx, client.ObjectKey{Name: desired.GetName(), Namespace: desired.GetNamespace()}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	objectExists := err == nil

	if objectExists {
		logger.Info("Object already exists. Updating", "Name", existing.GetName(), "Namespace", existing.GetNamespace())

		desired.SetResourceVersion(existing.GetResourceVersion())
		// TODO : Copy all the fields from the existing
		desired.SetFinalizers(desired.GetFinalizers())
		err = apiClient.Update(ctx, desired)
		if err != nil {
			return fmt.Errorf("failed to update object %s: %w", existing.GetName(), err)
		}

		return nil
	}

	logger.Info("Creating object", "Name", desired.GetName(), "Namespace", desired.GetNamespace())
	err = apiClient.Create(ctx, desired)
	if err != nil {
		return fmt.Errorf("failed to create object %s: %w", desired.GetName(), err)
	}

	return nil
}

func reconcileObjects(ctx context.Context, logger logr.Logger, apiClient client.Client,
	objects []client.Object, lister ItemsLister) error {

	objectsToUninstall, err := listInstalledObjects(ctx, logger, apiClient, lister)
	if err != nil {
		return err
	}

	for _, o := range objects {
		if o.GetNamespace() != "" {
			err = utils.CreateNamespaceIfNotExist(apiClient, ctx, logger, o.GetNamespace())
			if err != nil {
				return fmt.Errorf("unable to create object namespace: %w", err)
			}
		}

		logger.Info("Reconciling object", "Name", o.GetName(), "Namespace", o.GetNamespace())
		err = createOrUpdateObject(ctx, logger, apiClient, o)
		if err != nil {
			logger.Error(err, "Failed to reconcile object", "Name", o.GetName(), "Namespace", o.GetNamespace())
			return err
		}

		// if the object is in the spec, we shouldn't uninstall it
		delete(objectsToUninstall, generateName(o))
	}

	if len(objectsToUninstall) > 0 {
		err = deleteObjects(ctx, logger, apiClient, objectsToUninstall)
		if err != nil {
			return err
		}
	}

	return nil
}
