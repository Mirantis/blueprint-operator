package controllers

import (
	"context"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
)

const (
	blueprintName = "test-blueprint"
)

var blueprintLookupKey = types.NamespacedName{Name: blueprintName, Namespace: consts.NamespaceBlueprintSystem}

func newBlueprint(addons ...v1alpha1.AddonSpec) *v1alpha1.Blueprint {
	blueprint := &v1alpha1.Blueprint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "blueprint.mirantis.com/v1alpha1",
			Kind:       "Blueprint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      blueprintName,
			Namespace: consts.NamespaceBlueprintSystem,
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

			key := types.NamespacedName{Name: blueprintName, Namespace: consts.NamespaceBlueprintSystem}
			Eventually(getObject(ctx, key, blueprint), defaultTimeout, defaultInterval).Should(BeTrue())
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
				Enabled:   true,
				Kind:      "chart",
				Chart: &v1alpha1.ChartInfo{
					Name:    "nginx",
					Repo:    "https://charts.bitnami.com/bitnami",
					Version: "16.0.0",
				},
			}

			addonKey = types.NamespacedName{Name: addonName, Namespace: consts.NamespaceBlueprintSystem}

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
				Eventually(getObject(ctx, addonKey, actual), defaultTimeout, defaultInterval).Should(BeTrue())
				assertAddon(helmAddon, actual.Spec)
			})
		})

		//Context("Helm chart addon is removed from blueprint", func() {
		//	It("Should delete addon resource", func() {
		//		By("Creating a blueprint with one addon")
		//		blueprint := newBlueprint(helmAddon)
		//		Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())
		//
		//		By("Waiting for addon to be created")
		//		actual := &v1alpha1.Addon{}
		//		Eventually(getObject(ctx, addonKey, actual), defaultTimeout, defaultInterval).Should(BeTrue())
		//		assertAddon(helmAddon, actual.Spec)
		//
		//		By("Removing addon from blueprints")
		//		empty := newBlueprint()
		//		Expect(createOrUpdateBlueprint(ctx, empty)).Should(Succeed())
		//
		//		By("Checking if addon is removed")
		//		createdAddon := &v1alpha1.Addon{}
		//		Eventually(getObject(ctx, addonKey, createdAddon), defaultTimeout, defaultInterval).Should(BeFalse())
		//	})
		//})
	})
})

var _ = Describe("Object operations", func() {
	Context("issuers", func() {
		It("only lists BOP managed issuers", func(ctx context.Context) {
			fakeClient := fake.NewClientBuilder().WithObjects(
				&v1.Issuer{ObjectMeta: metav1.ObjectMeta{
					Name:      "issuer1",
					Namespace: "ns1",
					Labels:    map[string]string{consts.ManagedByLabel: consts.ManagedByValue},
				}},
				&v1.Issuer{ObjectMeta: metav1.ObjectMeta{
					Name:      "issuer2",
					Namespace: "ns2",
					Labels:    nil,
				}},
			).Build()

			objs, err := listIssuers(ctx, fakeClient)
			Expect(err).To(BeNil())

			Expect(objs).To(HaveLen(1))
			Expect(objs[0].GetName()).To(Equal("issuer1"))
		})

		It("creates issuer with BOP managed label", func(ctx context.Context) {
			issuer := issuerObject(v1alpha1.Issuer{Name: "issuer1", Namespace: "ns1"})
			Expect(issuer.GetLabels()[consts.ManagedByLabel]).To(Equal(consts.ManagedByValue))
		})
	})

	Context("cluster issuers", func() {
		It("only lists BOP managed cluster issuers", func(ctx context.Context) {
			fakeClient := fake.NewClientBuilder().WithObjects(
				&v1.ClusterIssuer{ObjectMeta: metav1.ObjectMeta{
					Name:   "clusterissuer1",
					Labels: map[string]string{consts.ManagedByLabel: consts.ManagedByValue},
				}},
				&v1.ClusterIssuer{ObjectMeta: metav1.ObjectMeta{
					Name:   "clusterissuer2",
					Labels: nil,
				}},
			).Build()

			objs, err := listClusterIssuers(ctx, fakeClient)
			Expect(err).To(BeNil())

			Expect(objs).To(HaveLen(1))
			Expect(objs[0].GetName()).To(Equal("clusterissuer1"))
		})

		It("creates cluster issuer with BOP managed label", func(ctx context.Context) {
			issuer := clusterIssuerObject(v1alpha1.ClusterIssuer{Name: "clusterissuer1"})
			Expect(issuer.GetLabels()[consts.ManagedByLabel]).To(Equal(consts.ManagedByValue))
		})
	})

	Context("certificates", func() {
		It("only lists BOP managed certificates", func(ctx context.Context) {
			fakeClient := fake.NewClientBuilder().WithObjects(
				&v1.Certificate{ObjectMeta: metav1.ObjectMeta{
					Name:   "certificate1",
					Labels: map[string]string{consts.ManagedByLabel: consts.ManagedByValue},
				}},
				&v1.Certificate{ObjectMeta: metav1.ObjectMeta{
					Name:   "certificate2",
					Labels: nil,
				}},
			).Build()

			objs, err := listCertificates(ctx, fakeClient)
			Expect(err).To(BeNil())

			Expect(objs).To(HaveLen(1))
			Expect(objs[0].GetName()).To(Equal("certificate1"))
		})

		It("creates certificate with BOP managed label", func(ctx context.Context) {
			issuer := certificateObject(v1alpha1.Certificate{Name: "certificate1"})
			Expect(issuer.GetLabels()[consts.ManagedByLabel]).To(Equal(consts.ManagedByValue))
		})
	})
})
