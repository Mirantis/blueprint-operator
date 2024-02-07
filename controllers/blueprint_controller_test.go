package controllers

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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

	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	BeforeAll(func() {
		// Start component installation by creating Installation CRD
		// This is needed for delete addon tests to work
		By("Creating Installation CRD")
		installation := &v1alpha1.Installation{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "boundless.mirantis.com/v1alpha1",
				Kind:       "Installation",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, installation)).Should(Succeed())

		By("Waiting for helm-controller to be created")
		dep := &appsv1.Deployment{}
		lookupKey := types.NamespacedName{Name: "helm-controller", Namespace: NamespaceBoundlessSystem}
		Eventually(getObject(ctx, lookupKey, dep), timeout, interval).Should(BeTrue())
	})

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
			Eventually(getObject(ctx, key, blueprint), timeout, interval).Should(BeTrue())
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
				Eventually(getObject(ctx, addonKey, actual), timeout, interval).Should(BeTrue())
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
				Eventually(getObject(ctx, addonKey, actual), timeout, interval).Should(BeTrue())
				assertAddon(helmAddon, actual.Spec)

				By("Removing addon from blueprints")
				empty := newBlueprint()
				Expect(createOrUpdateBlueprint(ctx, empty)).Should(Succeed())

				By("Checking if addon is removed")
				createdAddon := &v1alpha1.Addon{}
				Eventually(getObject(ctx, addonKey, createdAddon), timeout, interval).Should(BeFalse())
			})
		})
	})
})
