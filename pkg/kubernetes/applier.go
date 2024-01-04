package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Applier struct {
	log    logr.Logger
	client client.Client
}

func NewApplier(logger logr.Logger, client client.Client) *Applier {
	return &Applier{
		log:    logger,
		client: client,
	}
}

func (a *Applier) Apply(ctx context.Context, reader UnstructuredReader) error {
	var err error

	objs, err := a.readManifest(reader)
	if err != nil {
		return fmt.Errorf("failed to decode objects: %w", err)
	}

	// separate out the CRDs and other objects
	// CRDs need to be created first
	crds, others := a.splitCrdAndOthers(objs)
	a.log.Info("Found objects", "CRD", len(crds), "Other", len(others))
	for _, o := range crds {
		if err = a.createOrUpdateObject(ctx, o); err != nil {
			return fmt.Errorf("failed to apply crds resources from manifest: %w", err)
		}
	}

	// @todo wait for crds to be available before creating other objects

	// create other objects
	for _, o := range others {
		//client.MergeFrom(o)
		if err = a.createOrUpdateObject(ctx, o); err != nil {
			return fmt.Errorf("failed to apply resources from manifest at: %w", err)
		}
	}
	return nil
}

func (a *Applier) Delete(ctx context.Context, objs []unstructured.Unstructured) error {
	for _, o := range objs {
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(o.GroupVersionKind())
		key := client.ObjectKey{
			Namespace: o.GetNamespace(),
			Name:      o.GetName(),
		}
		if err := a.client.Get(ctx, key, existing); err != nil {
			if apierrors.IsNotFound(err) {
				a.log.Error(err, "Already delete", "Namespace", o.GetNamespace(), "Name", o.GetName())
				continue
			}
			return fmt.Errorf("failed to delete object: %s/%s", o.GetNamespace(), o.GetName())
		}
		a.log.Info("Deleting object", "Kind", existing.GetKind(), "Namespace", existing.GetNamespace(), "Name", existing.GetName())
		if err := a.client.Delete(ctx, existing); err != nil {
			return fmt.Errorf("failed to delete %s/%s/%s", existing.GetKind(), existing.GetNamespace(), existing.GetName())
		}
	}
	return nil
}

func (a *Applier) splitCrdAndOthers(objs []*unstructured.Unstructured) ([]*unstructured.Unstructured, []*unstructured.Unstructured) {
	var crds []*unstructured.Unstructured
	var others []*unstructured.Unstructured
	for _, o := range objs {
		if o.GetKind() == "CustomResourceDefinition" {
			crds = append(crds, o)
		} else {
			others = append(others, o)
		}
	}
	return crds, others
}

func (a *Applier) createOrUpdateObject(ctx context.Context, obj *unstructured.Unstructured) error {
	name := obj.GetName()
	kind := obj.GetKind()

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(obj.GroupVersionKind())

	key := client.ObjectKeyFromObject(obj)
	a.log.Info("Checking if object with key exists", "Key", key)
	err := a.client.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		a.log.Info("Creating object", "Kind", kind, "Name", name)
		if err = a.client.Create(ctx, obj); err != nil {
			return fmt.Errorf("failed to create resource %q of kind %q: %w", name, kind, err)
		}
		a.log.Info("Created object", "Kind", kind, "Name", name)
	} else {
		a.log.Info("Updating object", "Kind", kind, "Name", name)
		obj.SetResourceVersion(existing.GetResourceVersion())
		if err = a.client.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to update resource %q of kind %q: %w", name, kind, err)
		}
		a.log.Info("Updated object", "Kind", kind, "Name", name)
	}

	return nil
}

func (a *Applier) readManifest(r UnstructuredReader) ([]*unstructured.Unstructured, error) {
	var o []*unstructured.Unstructured
	var errs error
	for {
		obj, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors.Join(errs, fmt.Errorf("could not read object: %w", err))
			continue
		}
		if obj == nil {
			continue
		}

		o = append(o, obj)
	}

	return o, errs
}
