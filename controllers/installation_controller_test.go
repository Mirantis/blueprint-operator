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
// This is because all these tests runs in a single "environment"
var _ = Describe("Testing installation", Ordered, Serial, func() {
	Context("Reconcile tests", func() {
		It("Should install Installation", func() {
			lookupKey := types.NamespacedName{Name: DefaultInstanceKey.Name, Namespace: DefaultInstanceKey.Namespace}
			Eventually(getObject(context.TODO(), lookupKey, &operator.Installation{}), DefaultTimeout, DefaultInterval).Should(BeTrue())
		})
		It("Should install Helm Controller", func() {
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
})
