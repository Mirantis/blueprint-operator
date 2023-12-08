package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mirantis/boundless-operator/api/v1alpha1"
)

const (
	blueprintName = "test-blueprint"
)

var blueprintLookupKey = types.NamespacedName{Name: blueprintName, Namespace: NamespaceBoundlessSystem}

func generateBlueprint(addons ...v1alpha1.AddonSpec) *v1alpha1.Blueprint {
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

	Context("A blueprint is created", func() {

		It("Should successfully be created", func() {
			blueprint := generateBlueprint()
			Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())

			key := types.NamespacedName{Name: blueprintName, Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(ctx, key, blueprint), timeout, interval).Should(BeTrue())
		})

		It("Should install Helm Controller", func() {
			ctx := context.Background()
			helmDeploy := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "helm-controller", Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(ctx, lookupKey, helmDeploy), timeout, interval).Should(BeTrue())
		})
	})

	Context("A blueprint is updated", func() {
		Context("A helm addon", func() {
			const (
				addonName      = "test-addon"
				addonNamespace = "test-ns"
			)

			nginxHelmAddon := v1alpha1.AddonSpec{
				Name:      addonName,
				Namespace: addonNamespace,
				Kind:      "chart",
				Chart: &v1alpha1.ChartInfo{
					Name:    "nginx",
					Repo:    "https://charts.bitnami.com/bitnami",
					Version: "15.1.1",
				},
			}

			Context("Is added to the blueprint", func() {

				It("Should be updated successfully", func() {
					blueprint := generateBlueprint(nginxHelmAddon)
					Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())
				})

				It("Should create the addon resource", func() {

					lookupKey := types.NamespacedName{Name: addonName, Namespace: NamespaceBoundlessSystem}
					createdAddon := &v1alpha1.Addon{}
					Eventually(getObject(ctx, lookupKey, createdAddon), timeout, interval).Should(BeTrue())
					Expect(createdAddon.Spec.Name).Should(Equal(addonName))
				})

				It("Should create namespace specified by the addon", func() {
					ctx := context.Background()
					ns := &v1.Namespace{}
					key := types.NamespacedName{Name: addonNamespace}
					Eventually(getObject(ctx, key, ns)).Should(BeTrue())
				})
			})

			Context("Is removed from blueprint", func() {

				It("Should be updated successfully", func() {
					blueprint := generateBlueprint()
					Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())
				})

				It("Should remove the addon resource", func() {
					lookupKey := types.NamespacedName{Name: addonName, Namespace: NamespaceBoundlessSystem}
					createdAddon := &v1alpha1.Addon{}
					Eventually(getObject(ctx, lookupKey, createdAddon), timeout, interval).Should(BeFalse())
				})
			})
		})
	})
})
