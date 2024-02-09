package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

const (
	// Default timeout for waiting for assertions
	DefaultTimeout = time.Second * 10

	// Default interval for waiting for assertions
	DefaultInterval = time.Millisecond * 250
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

func assertAddon(expected, actual v1alpha1.AddonSpec) {
	GinkgoHelper()

	Expect(actual.Name).Should(Equal(expected.Name))
	Expect(actual.Namespace).Should(Equal(expected.Namespace))
	Expect(actual.Kind).Should(Equal(expected.Kind))

	if expected.Kind == "chart" {
		Expect(actual.Manifest).Should(BeNil())

		Expect(actual.Chart).ShouldNot(BeNil())
		Expect(actual.Chart.Name).Should(Equal(expected.Chart.Name))
		Expect(actual.Chart.Repo).Should(Equal(expected.Chart.Repo))
		Expect(actual.Chart.Version).Should(Equal(expected.Chart.Version))
	} else {
		Expect(actual.Chart).Should(BeNil())

		Expect(actual.Manifest).ShouldNot(BeNil())
		Expect(actual.Manifest.URL).Should(Equal(expected.Manifest.URL))
	}
}

func containsAddon(list []v1alpha1.AddonSpec, ns, name string) bool {
	for _, a := range list {
		if a.Namespace == ns && a.Name == name {
			return true
		}
	}
	return false
}

func randomName(pre string) string {
	return envconf.RandomName(pre, 10)
}
