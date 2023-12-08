package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mirantis/boundless-operator/api/v1alpha1"
)

var _ = Describe("Addon controller", func() {
	const (
		AddonName      = "test-addon2"
		AddonNamespace = "test-ns"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("Addon of kind Helm is created", func() {
		ctx := context.Background()
		addon := &v1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name:      AddonName,
				Namespace: NamespaceBoundlessSystem,
			},
			Spec: v1alpha1.AddonSpec{
				Name:      AddonName,
				Kind:      "chart",
				Enabled:   true,
				Namespace: AddonNamespace,
				Chart: &v1alpha1.ChartInfo{
					Name:    "nginx",
					Repo:    "https://charts.bitnami.com/bitnami",
					Version: "15.1.1",
				},
			},
		}

		It("Should create HelmAddon", func() {
			Expect(k8sClient.Create(ctx, addon)).Should(Succeed())

			createdAddon := &v1alpha1.Addon{}
			key := types.NamespacedName{Name: AddonName, Namespace: NamespaceBoundlessSystem}
			Eventually(getObject(ctx, key, createdAddon), timeout, interval).Should(BeTrue())
			Expect(createdAddon.Spec.Name).Should(Equal(AddonName))
		})

		It("Should create HelmChart CRD", func() {
			//@todo
		})

		It("Should have the expected status", func() {
			//@todo
		})
	})
})
