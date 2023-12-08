package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"

	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantis/boundless-operator/api/v1alpha1"
)

func getObject(ctx context.Context, key runtimeclient.ObjectKey, obj runtimeclient.Object) func() bool {
	GinkgoHelper()
	return func() bool {
		if err := k8sClient.Get(ctx, key, obj); err != nil {
			return false
		}
		return true
	}
}

func createOrUpdateBlueprint(ctx context.Context, new *v1alpha1.Blueprint) error {
	GinkgoHelper()

	cp := new.DeepCopy()
	existing := &v1alpha1.Blueprint{}
	_ = k8sClient.Get(ctx, blueprintLookupKey, existing)
	if existing.Name != "" {
		// Copy addons from new object to existing
		// as addons are the only changes we make in these tests
		// This is to prevent issue caused by overwriting existing fields
		// set by controllers on the object
		existing.Spec.Components.Addons = cp.Spec.Components.Addons
		return k8sClient.Update(ctx, existing)
	}
	return k8sClient.Create(ctx, cp)
}
