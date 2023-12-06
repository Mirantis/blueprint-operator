package controllers

//
//import (
//	"context"
//	"time"
//
//	. "github.com/onsi/ginkgo/v2"
//	. "github.com/onsi/gomega"
//	appsv1 "k8s.io/api/apps/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"k8s.io/apimachinery/pkg/types"
//
//	"github.com/mirantis/boundless-operator/api/v1alpha1"
//)
//
//var _ = Describe("Addon controller", func() {
//
//	// Define utility constants for object names and testing timeouts/durations and intervals.
//	const (
//		AddonName         = "test-addon2"
//		AddonWorkloadName = "nginx"
//		AddonNamespace    = "test-ns"
//
//		timeout  = time.Second * 10
//		duration = time.Second * 10
//		interval = time.Millisecond * 250
//	)
//
//	Context("When creating an addon", func() {
//
//		It("Should create create HelmAddon", func() {
//			By("By creating a addon of kind helm")
//
//			ctx := context.Background()
//			addon := &v1alpha1.Addon{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      AddonName,
//					Namespace: NamespaceBoundlessSystem,
//				},
//				Spec: v1alpha1.AddonSpec{
//					Name:      AddonName,
//					Kind:      "chart",
//					Enabled:   true,
//					Namespace: AddonNamespace,
//					Chart: &v1alpha1.ChartInfo{
//						Name:    "nginx",
//						Repo:    "https://charts.bitnami.com/bitnami",
//						Version: "15.1.1",
//					},
//				},
//			}
//
//			Expect(k8sClient.Create(ctx, addon)).Should(Succeed())
//
//			createdAddon := &v1alpha1.Addon{}
//			createdDeployment := &appsv1.Deployment{}
//
//			Eventually(func() bool {
//				lookupKey := types.NamespacedName{Name: AddonName, Namespace: NamespaceBoundlessSystem}
//				err := k8sClient.Get(ctx, lookupKey, createdAddon)
//				if err != nil {
//					return false
//				}
//				return true
//			}, timeout, interval).Should(BeTrue())
//
//			Eventually(func() bool {
//				lookupKey := types.NamespacedName{Name: AddonWorkloadName, Namespace: AddonNamespace}
//				err := k8sClient.Get(ctx, lookupKey, createdAddon)
//				if err != nil {
//					return false
//				}
//				return true
//			}, timeout, interval).Should(BeTrue())
//
//			Expect(createdDeployment.Name).Should(Equal(AddonWorkloadName))
//		})
//	})
//})
