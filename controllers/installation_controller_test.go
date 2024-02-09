package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	operator "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

// These tests should run in the serial (not parallel) and in order specified
// Otherwise, the results may not be predictable
// This is because all these tests runs in a single environment
var _ = FDescribe("Testing installation controller", Ordered, Serial, func() {
	Context("Reconcile tests", func() {
		It("Finalizer should be added", func() {
			obj := &operator.Installation{}
			lookupKey := types.NamespacedName{Name: DefaultInstanceKey.Name, Namespace: DefaultInstanceKey.Namespace}

			Eventually(func() []string {
				err := k8sClient.Get(context.TODO(), lookupKey, obj)
				Expect(err).NotTo(HaveOccurred())
				return obj.Finalizers
			}, DefaultTimeout, DefaultInterval).Should(ContainElement(installationFinalizer))

			Expect(k8sClient.Get(context.TODO(), lookupKey, obj)).Should(Succeed())
			Expect(obj.Finalizers).Should(ContainElement(installationFinalizer))
		})
		It("Should create Installation resource", func() {
			lookupKey := types.NamespacedName{Name: DefaultInstanceKey.Name, Namespace: DefaultInstanceKey.Namespace}
			Eventually(getObject(context.TODO(), lookupKey, &operator.Installation{}), DefaultTimeout, DefaultInterval).Should(BeTrue())
		})
		It("Should install helm controller", func() {
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "helm-controller", Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), DefaultTimeout, DefaultInterval).Should(BeTrue())
		})

		It("Should install cert manager", func() {
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "cert-manager", Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), DefaultTimeout, DefaultInterval).Should(BeTrue())
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
			lookupKey := types.NamespacedName{Name: "helm-controller", Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), DefaultTimeout, DefaultInterval).Should(BeFalse())
		})
		It("Should delete cert manager", func() {
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "cert-manager", Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(context.TODO(), lookupKey, dep), DefaultTimeout, DefaultInterval).Should(BeFalse())
		})
	})
})
