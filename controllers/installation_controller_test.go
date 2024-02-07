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
)

// These tests should run in the serial (not parallel) and in order specified
// Otherwise, the results may not be predictable
// This is because all these tests runs in a single "environment"
var _ = Describe("Testing installation", Ordered, Serial, func() {

	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("Reconcile tests", func() {
		BeforeAll(func() {
			i := &operator.Installation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "boundless.mirantis.com/v1alpha1",
					Kind:       "Installation",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: metav1.NamespaceDefault,
				},
			}

			Expect(k8sClient.Create(ctx, i)).Should(Succeed())
		})
		It("Should install Helm Controller", func() {
			ctx := context.Background()
			dep := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "helm-controller", Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(ctx, lookupKey, dep), timeout, interval).Should(BeTrue())
		})

		It("Should install cert manager", func() {
			ctx := context.Background()
			helmDeploy := &appsv1.Deployment{}
			lookupKey := types.NamespacedName{Name: "cert-manager", Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(ctx, lookupKey, helmDeploy), timeout, interval).Should(BeTrue())
		})
	})
})
