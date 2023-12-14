package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Apply decodes the manifest data and create or update the objects
func Apply(ctx context.Context, log logr.Logger, client client.Client, data []byte) error {
	var err error

	objs, err := decodeObjects(data)
	if err != nil {
		return fmt.Errorf("failed to decode objects: %w", err)
	}
	// separate out the CRDs and other objects
	// CRDs need to be created first
	crds, others := splitCrdAndOthers(objs)
	log.Info("Found objects", "CRD", len(crds), "Other", len(others))
	for _, o := range crds {
		if err = createOrUpdateObject(ctx, log, client, &o); err != nil {
			return fmt.Errorf("failed to apply crds resources from manifest: %w", err)
		}
	}

	// @todo wait for crds to be available before creating other objects

	// create other objects
	for _, o := range others {
		if err = createOrUpdateObject(ctx, log, client, &o); err != nil {
			return fmt.Errorf("failed to apply resources from manifest at: %w", err)
		}
	}
	return nil
}

// Delete deletes the provided objects
func Delete(ctx context.Context, log logr.Logger, c client.Client, objs []unstructured.Unstructured) error {
	for _, o := range objs {
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(o.GroupVersionKind())
		key := client.ObjectKey{
			Namespace: o.GetNamespace(),
			Name:      o.GetName(),
		}
		if err := c.Get(ctx, key, existing); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Already delete", "Namespace", o.GetNamespace(), "Name", o.GetName())
				continue
			}
			return fmt.Errorf("failed to delete object: %s/%s", o.GetNamespace(), o.GetName())
		}
		log.Info("Deleting object", "Kind", existing.GetKind(), "Namespace", existing.GetNamespace(), "Name", existing.GetName())
		if err := c.Delete(ctx, existing); err != nil {
			return fmt.Errorf("failed to delete %s/%s/%s", existing.GetKind(), existing.GetNamespace(), existing.GetName())
		}
	}
	return nil
}

func splitCrdAndOthers(objs []unstructured.Unstructured) ([]unstructured.Unstructured, []unstructured.Unstructured) {
	var crds []unstructured.Unstructured
	var others []unstructured.Unstructured
	for _, o := range objs {
		if o.GetKind() == "CustomResourceDefinition" {
			crds = append(crds, o)
		} else {
			others = append(others, o)
		}
	}
	return crds, others
}

func createOrUpdateObject(ctx context.Context, log logr.Logger, c client.Client, obj *unstructured.Unstructured) error {
	name := obj.GetName()
	kind := obj.GetKind()

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(obj.GroupVersionKind())

	key := client.ObjectKeyFromObject(obj)
	log.Info("Checking if object with key exists", "Key", key)
	err := c.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		log.Info("Creating object", "Kind", kind, "Name", name)
		if err = c.Create(ctx, obj); err != nil {
			return fmt.Errorf("failed to create resource %q of kind %q: %w", name, kind, err)
		}
		log.Info("Created object", "Kind", kind, "Name", name)
	} else {
		log.Info("Updating object", "Kind", kind, "Name", name)
		obj.SetResourceVersion(existing.GetResourceVersion())
		if err = c.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to update resource %q of kind %q: %w", name, kind, err)
		}
		log.Info("Updated object", "Kind", kind, "Name", name)
	}

	return nil
}

func decodeObjects(data []byte) ([]unstructured.Unstructured, error) {
	var objs []unstructured.Unstructured
	decoder := yaml.NewYAMLToJSONDecoder(bytes.NewReader(data))

	var o unstructured.Unstructured
	for {
		if err := decoder.Decode(&o); err != nil {
			if err != io.EOF {
				return objs, fmt.Errorf("error decoding yaml manifest file: %s", err)
			}
			break
		}
		objs = append(objs, o)

	}
	return objs, nil
}
