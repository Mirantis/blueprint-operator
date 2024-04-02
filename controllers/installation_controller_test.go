package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operator "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
)

// These tests should run in the serial (not parallel) and in order specified
// Otherwise, the results may not be predictable
// This is because all these tests runs in a single environment
var _ = Describe("Testing installation controller", Ordered, Serial, func() {
	Context("Reconcile tests", func() {
		It("Finalizer should be added", func() {
			obj := &operator.Installation{}
			lookupKey := types.NamespacedName{Name: DefaultInstanceKey.Name, Namespace: DefaultInstanceKey.Namespace}

			// getFinalizers returns the finalizers of the object
			getFinalizers := func() []string {
				err := k8sClient.Get(context.TODO(), lookupKey, obj)
				Expect(err).NotTo(HaveOccurred())
				return obj.Finalizers
			}
			Eventually(getFinalizers, defaultTimeout, defaultInterval).Should(ContainElement(installationFinalizer))

			Expect(k8sClient.Get(context.TODO(), lookupKey, obj)).Should(Succeed())
			Expect(obj.Finalizers).Should(ContainElement(installationFinalizer))
		})
		It("Should create Installation resource", func() {
			lookupKey := types.NamespacedName{Name: DefaultInstanceKey.Name, Namespace: DefaultInstanceKey.Namespace}
			Eventually(getObject(context.TODO(), lookupKey, &operator.Installation{}), defaultTimeout, defaultInterval).Should(BeTrue())
		})
		It("Should install helm controller", func() {
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "helm-controller", Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), defaultTimeout, defaultInterval).Should(BeTrue())
		})

		It("Should install cert manager", func() {
			dep := &appsv1.Deployment{}

			By("Checking cert-manager deployment")
			lookupKey := types.NamespacedName{Name: "cert-manager", Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), defaultTimeout, defaultInterval).Should(BeTrue())

			By("Checking cert-manager-webhook deployment")
			lookupKey = types.NamespacedName{Name: "cert-manager-webhook", Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), defaultTimeout, defaultInterval).Should(BeTrue())

			By("Checking cert-manager-cainjector deployment")
			lookupKey = types.NamespacedName{Name: "cert-manager-cainjector", Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), defaultTimeout, defaultInterval).Should(BeTrue())

		})
		It("Should install webhook", func() {
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "boundless-operator-webhook", Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), time.Minute*5, defaultInterval).Should(BeTrue())
		})
	})

	Context("When Installation resource is deleted", func() {
		BeforeAll(func() {
			// Delete the Installation
			install := &operator.Installation{}
			lookupKey := types.NamespacedName{Name: DefaultInstanceKey.Name, Namespace: DefaultInstanceKey.Namespace}
			Expect(k8sClient.Get(context.TODO(), lookupKey, install)).Should(Succeed())
			Expect(k8sClient.Delete(context.TODO(), install)).Should(Succeed())
		})
		It("Should delete Helm Controller", func() {
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "helm-controller", Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), timeoutOneMinute, defaultInterval).Should(BeFalse())
		})
		It("Should delete cert manager", func() {
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "cert-manager", Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), timeoutOneMinute, defaultInterval).Should(BeFalse())
		})
		AfterAll(func() {
			// Create the Installation again to avoid the error in the next tests
			install := &operator.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultInstanceKey.Name,
					Namespace: DefaultInstanceKey.Namespace,
				},
			}
			Expect(k8sClient.Create(context.TODO(), install)).Should(Succeed())
			dep := &appsv1.Deployment{}
			helmKey := types.NamespacedName{Name: "helm-controller", Namespace: consts.NamespaceBoundlessSystem}
			certKey := types.NamespacedName{Name: "cert-manager", Namespace: consts.NamespaceBoundlessSystem}

			Eventually(getObject(context.TODO(), helmKey, dep), defaultTimeout, defaultInterval).Should(BeTrue(), "Failed to reinstall helm controller")
			Eventually(getObject(context.TODO(), certKey, dep), defaultTimeout, defaultInterval).Should(BeTrue(), "Failed to reinstall cert manager")
		})
	})
})
