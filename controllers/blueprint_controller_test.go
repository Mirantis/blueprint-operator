package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mirantis/boundless-operator/api/v1alpha1"
)

var _ = Describe("Blueprint controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		Namespace     = NamespaceBoundlessSystem
		BlueprintName = "test-blueprint"
		AddonName     = "test-addon"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	BeforeEach(func() {
		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: NamespaceBoundlessSystem,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
	})

	Context("When creating a blueprint", func() {

		It("Should create the specified addon", func() {
			By("By creating a new Blueprint")

			ctx := context.Background()
			blueprint := &v1alpha1.Blueprint{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "boundless.mirantis.com/v1alpha1",
					Kind:       "Blueprint",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlueprintName,
					Namespace: Namespace,
				},
				Spec: v1alpha1.BlueprintSpec{
					Components: v1alpha1.Component{
						Addons: []v1alpha1.AddonSpec{
							{
								Name:      AddonName,
								Kind:      "chart",
								Namespace: "test-ns",
								Chart: &v1alpha1.ChartInfo{
									Name:    "nginx",
									Repo:    "https://charts.bitnami.com/bitnami",
									Version: "15.1.1",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, blueprint)).Should(Succeed())

			createdBlueprint := &v1alpha1.Blueprint{}
			createdAddon := &v1alpha1.Addon{}

			Eventually(func() bool {
				lookupKey := types.NamespacedName{Name: BlueprintName, Namespace: Namespace}
				err := k8sClient.Get(ctx, lookupKey, createdBlueprint)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				lookupKey := types.NamespacedName{Name: AddonName, Namespace: Namespace}
				err := k8sClient.Get(ctx, lookupKey, createdAddon)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(len(createdBlueprint.Spec.Components.Addons)).Should(Equal(1))
			Expect(createdAddon.Spec.Name).Should(Equal(AddonName))
		})
	})
})
