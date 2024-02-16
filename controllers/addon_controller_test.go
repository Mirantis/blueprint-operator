package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
)

func newAddon(addons ...v1alpha1.AddonSpec) v1alpha1.Addon {
	addon := v1alpha1.Addon{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "boundless.mirantis.com/v1alpha1",
			Kind:       "Addon",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "addon-1",
			Namespace: consts.NamespaceBoundlessSystem,
		},
		Spec: v1alpha1.AddonSpec{
			Name:      "addon-1",
			Namespace: "ns-1",
			Kind:      "chart",
			Chart: &v1alpha1.ChartInfo{
				Name:    "nginx",
				Repo:    "https://charts.bitnami.com/bitnami",
				Version: "15.1.1",
			},
		},
	}

	return addon
}

// These tests should run in the serial (not parallel) and in order specified
// Otherwise, the results may not be predictable
// This is because all these tests runs in a single "environment"
var _ = Describe("Addon controller", Ordered, Serial, func() {

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

	var (
		scheme     *runtime.Scheme
		fakeClient client.Client
		reconciler *AddonReconciler
	)

	Context("Helm Addon", func() {
		var addon v1alpha1.Addon
		var namespacedName types.NamespacedName
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(v1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			reconciler = &AddonReconciler{
				Client: fakeClient,
				Scheme: fakeClient.Scheme(),
			}

			namespacedName = types.NamespacedName{Name: "addon-1", Namespace: consts.NamespaceBoundlessSystem}
			addon = v1alpha1.Addon{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "boundless.mirantis.com/v1alpha1",
					Kind:       "Addon",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: v1alpha1.AddonSpec{
					Name:      "addon-1",
					Namespace: "ns-1",
					Kind:      "chart",
					Chart: &v1alpha1.ChartInfo{
						Name:    "nginx",
						Repo:    "https://charts.bitnami.com/bitnami",
						Version: "15.1.1",
					},
				},
			}
		})

		It("Should fail if kind is incorrect", func() {
			addon.Spec.Kind = "incorrect"
			Expect(fakeClient.Create(context.TODO(), &addon)).Should(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: namespacedName})
			Expect(result).Should(Equal(ctrl.Result{Requeue: false}), "Requeue should be false")
			Expect(err).To(BeNil(), "Error should be nil")
		})

		It("Should fail if chart is incorrect", func() {
			addon.Spec.Chart = nil
			Expect(fakeClient.Create(context.TODO(), &addon)).Should(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: namespacedName})
			Expect(result).Should(Equal(ctrl.Result{Requeue: false}))
			Expect(err).To(BeNil())
		})

	})

	//Context("Helm Addon", func() {
	//	var helmAddon v1alpha1.Addon
	//
	//	When("An addon is created", func() {
	//		BeforeAll(func() {
	//			helmAddon = newAddon()
	//			Expect(k8sClient.Create(ctx, &helmAddon)).Should(Succeed())
	//		})
	//		It("Should successfully be created", func() {
	//			actual := v1alpha1.Addon{}
	//			key := types.NamespacedName{Name: "addon-1", Namespace: consts.NamespaceBoundlessSystem}
	//			Eventually(getObject(ctx, key, &actual), defaultTimeout, defaultInterval).Should(BeTrue())
	//		})
	//		It("Correctly creates HelmChart resource", func() {
	//			hc := helmv1.HelmChart{}
	//			key := types.NamespacedName{Name: "addon-1", Namespace: consts.NamespaceBoundlessSystem}
	//
	//			By("Checking HelmChart resource is created")
	//			Eventually(getObject(ctx, key, &hc), defaultTimeout, defaultInterval).Should(BeTrue())
	//
	//			By("Checking HelmChart resource has the correct values")
	//			Expect(hc.Spec.Chart).Should(Equal("https://charts.bitnami.com/bitnami/nginx"))
	//			Expect(hc.Spec.Version).Should(Equal("15.1.1"))
	//			Expect(hc.Spec.TargetNamespace).Should(Equal("ns-1"))
	//		})
	//		AfterAll(func() {
	//			Expect(k8sClient.Delete(ctx, &helmAddon)).Should(Succeed())
	//		})
	//	})
	//
	//	Context("An addon is deleted", func() {
	//		BeforeAll(func() {
	//			helmAddon = newAddon()
	//			Expect(k8sClient.Create(ctx, &helmAddon)).Should(Succeed())
	//		})
	//
	//		It("Should successfully be deleted", func() {
	//			key := types.NamespacedName{Name: "addon-1", Namespace: consts.NamespaceBoundlessSystem}
	//			Expect(k8sClient.Delete(ctx, &helmAddon)).Should(Succeed())
	//			Eventually(getObject(ctx, key, &helmAddon), defaultTimeout, defaultInterval).Should(BeFalse())
	//		})
	//	})
	//})
})
