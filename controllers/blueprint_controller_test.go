package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

const (
	blueprintName = "test-blueprint"
)

var blueprintLookupKey = types.NamespacedName{Name: blueprintName, Namespace: NamespaceBoundlessSystem}

func newBlueprint(addons ...v1alpha1.AddonSpec) *v1alpha1.Blueprint {
	blueprint := &v1alpha1.Blueprint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "boundless.mirantis.com/v1alpha1",
			Kind:       "Blueprint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      blueprintName,
			Namespace: NamespaceBoundlessSystem,
		},
	}
	for _, addon := range addons {
		blueprint.Spec.Components.Addons = append(blueprint.Spec.Components.Addons, addon)
	}
	return blueprint
}

// These tests should run in the serial (not parallel) and in order specified
// Otherwise, the results may not be predictable
// This is because all these tests runs in a single "environment"
var _ = Describe("Blueprint controller", Ordered, Serial, func() {
	BeforeEach(func() {
		// Reset the state by creating empty blueprint
		blueprint := newBlueprint()
		Expect(k8sClient.Create(ctx, blueprint)).Should(Succeed())
	})

	AfterEach(func() {
		// Reset the state by deleting the blueprint
		blueprint := newBlueprint()
		Expect(k8sClient.Delete(ctx, blueprint)).Should(Succeed())
	})

	Context("A blueprint is created", func() {
		It("Should successfully be created", func() {
			blueprint := newBlueprint()
			Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())

			key := types.NamespacedName{Name: blueprintName, Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(ctx, key, blueprint), DefaultTimeout, DefaultInterval).Should(BeTrue())
		})
	})

	Context("A blueprint is updated", func() {
		var addonName, addonNamespace string
		var helmAddon v1alpha1.AddonSpec
		var addonKey types.NamespacedName

		BeforeEach(func() {
			addonName = randomName("addon")
			addonNamespace = randomName("ns")

			helmAddon = v1alpha1.AddonSpec{
				Name:      addonName,
				Namespace: addonNamespace,
				Kind:      "chart",
				Chart: &v1alpha1.ChartInfo{
					Name:    "nginx",
					Repo:    "https://charts.bitnami.com/bitnami",
					Version: "15.1.1",
				},
			}

			addonKey = types.NamespacedName{Name: addonName, Namespace: NamespaceBoundlessSystem}

		})
		Context("Helm chart addon is added to the blueprint", func() {
			BeforeEach(func() {
				By("Creating a blueprint with one addon")
				blueprint := newBlueprint(helmAddon)
				Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())
			})

			It("Should create blueprint with addon successfully", func() {
				b := &v1alpha1.Blueprint{}
				Eventually(getObject(ctx, blueprintLookupKey, b)).Should(BeTrue())
				Expect(containsAddon(b.Spec.Components.Addons, addonNamespace, addonName)).Should(BeTrue(), "addon %s/%s does not existing in the list", addonNamespace, addonName)
			})

			It("Should create the correct addon resource", func() {
				actual := &v1alpha1.Addon{}
				Eventually(getObject(ctx, addonKey, actual), DefaultTimeout, DefaultInterval).Should(BeTrue())
				assertAddon(helmAddon, actual.Spec)
			})
		})

		Context("Helm chart addon is removed from blueprint", func() {
			It("Should delete addon resource", func() {
				By("Creating a blueprint with one addon")
				blueprint := newBlueprint(helmAddon)
				Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())

				By("Waiting for addon to be created")
				actual := &v1alpha1.Addon{}
				Eventually(getObject(ctx, addonKey, actual), DefaultTimeout, DefaultInterval).Should(BeTrue())
				assertAddon(helmAddon, actual.Spec)

				By("Removing addon from blueprints")
				empty := newBlueprint()
				Expect(createOrUpdateBlueprint(ctx, empty)).Should(Succeed())

				By("Checking if addon is removed")
				createdAddon := &v1alpha1.Addon{}
				Eventually(getObject(ctx, addonKey, createdAddon), DefaultTimeout, DefaultInterval).Should(BeFalse())
			})
		})
	})
})
