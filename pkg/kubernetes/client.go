package kubernetes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/blueprint-operator/pkg/utils"
)

const (
	defaultFieldOwner = client.FieldOwner("blueprint-operator")
)

// Client is a wrapper around the controller-runtime client.Client
// that provides a higher-level API for working with unstructured objects.
type Client struct {
	log           logr.Logger
	runtimeClient client.Client
}

// NewClient creates a new Client
func NewClient(logger logr.Logger, runtimeClient client.Client) *Client {
	return &Client{log: logger, runtimeClient: runtimeClient}
}

// Apply applies the changes to the object in the cluster using server-side apply.
// If the object does not exist, it will be created using Create()
// If the object exists, it will be updated using Patch()
func (c *Client) Apply(ctx context.Context, obj client.Object) error {
	// Get the object from the cluster
	orig := obj.DeepCopyObject().(client.Object)
	if err := c.runtimeClient.Get(ctx, client.ObjectKeyFromObject(obj), orig); err != nil {
		if apierrors.IsNotFound(err) {
			// If the object does not exist, create it
			if err := c.create(ctx, obj); err != nil {
				return fmt.Errorf("failed to create the object %q: %w", utils.GetIdentifier(obj), err)
			}
			return nil
		}

		return fmt.Errorf("failed to get the object %q: %w", utils.GetIdentifier(obj), err)
	}

	// Perform server-side apply
	if err := c.runtimeClient.Patch(ctx, obj, client.Apply, client.ForceOwnership, defaultFieldOwner); err != nil {
		return fmt.Errorf("failed to apply the changes to object  %q: %w", utils.GetIdentifier(obj), err)
	}

	return nil
}

func (c *Client) create(ctx context.Context, obj client.Object) error {
	if err := c.runtimeClient.Create(ctx, obj); err != nil {
		return fmt.Errorf("failed to create the object: %w", err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, obj client.Object) error {
	if err := c.runtimeClient.Delete(ctx, obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			c.log.V(1).Info("object not found", "object", utils.GetIdentifier(obj))
			return nil
		}
		return fmt.Errorf("failed to delete the object %q: %w", utils.GetIdentifier(obj), err)
	}
	return nil
}

// ToUnstructured converts a runtime.Object into an Unstructured object.
func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	// If the incoming object is already unstructured, perform a deep copy first
	// otherwise DefaultUnstructuredConverter ends up returning the inner map without
	// making a copy.
	if _, ok := obj.(runtime.Unstructured); ok {
		obj = obj.DeepCopyObject()
	}
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: rawMap}, nil
}
