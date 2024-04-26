package controllers

import (
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
)

const (
	blueprintName = "test-blueprint"
)

var blueprintLookupKey = types.NamespacedName{Name: blueprintName, Namespace: consts.NamespaceBoundlessSystem}

func newBlueprint(caSpec *v1alpha1.CASpec, addons ...v1alpha1.AddonSpec) *v1alpha1.Blueprint {
	blueprint := &v1alpha1.Blueprint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "boundless.mirantis.com/v1alpha1",
			Kind:       "Blueprint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      blueprintName,
			Namespace: consts.NamespaceBoundlessSystem,
		},
	}
	for _, addon := range addons {
		blueprint.Spec.Components.Addons = append(blueprint.Spec.Components.Addons, addon)
	}
	if caSpec != nil {
		blueprint.Spec.Components.CAs = *caSpec
	}

	return blueprint
}

// These tests should run in the serial (not parallel) and in order specified
// Otherwise, the results may not be predictable
// This is because all these tests runs in a single "environment"
var _ = Describe("Blueprint controller", Ordered, Serial, func() {
	BeforeEach(func() {
		// Reset the state by creating empty blueprint
		blueprint := newBlueprint(nil)
		Expect(k8sClient.Create(ctx, blueprint)).Should(Succeed())
	})

	AfterEach(func() {
		// Reset the state by deleting the blueprint
		blueprint := newBlueprint(nil)
		Expect(k8sClient.Delete(ctx, blueprint)).Should(Succeed())
	})

	Context("A blueprint is created", func() {
		It("Should successfully be created", func() {
			blueprint := newBlueprint(nil)
			Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())

			key := types.NamespacedName{Name: blueprintName, Namespace: consts.NamespaceBoundlessSystem}
			Eventually(getObject(ctx, key, blueprint), defaultTimeout, defaultInterval).Should(BeTrue())
		})
	})

	Context("A blueprint is updated", func() {
		var addonName, addonNamespace, issuerName, issuerNamespace, clusterIssuerName string
		var caSpec v1alpha1.CASpec
		var helmAddon v1alpha1.AddonSpec
		var issuer, clusterIssuer certmanager.IssuerSpec
		var addonKey, issuerKey, clusterIssuerKey types.NamespacedName

		BeforeEach(func() {
			addonName = randomName("addon")
			addonNamespace = randomName("ns")

			issuerName = randomName("issuer")
			issuerNamespace = randomName("ns")

			clusterIssuerName = randomName("clusterissuer")

			helmAddon = v1alpha1.AddonSpec{
				Name:      addonName,
				Namespace: addonNamespace,
				Kind:      "chart",
				Chart: &v1alpha1.ChartInfo{
					Name:    "nginx",
					Repo:    "https://charts.bitnami.com/bitnami",
					Version: "15.1.1",
				},
			}

			issuer = certmanager.IssuerSpec{IssuerConfig: certmanager.IssuerConfig{
				CA: &certmanager.CAIssuer{
					SecretName: "ca-secret",
				},
			}}

			clusterIssuer = certmanager.IssuerSpec{IssuerConfig: certmanager.IssuerConfig{
				CA: &certmanager.CAIssuer{
					SecretName: "cluster-ca-secret",
				},
			}}

			caSpec = v1alpha1.CASpec{
				Issuers: []v1alpha1.Issuer{{
					Name:      issuerName,
					Namespace: issuerNamespace,
					Spec:      issuer,
				}},
				ClusterIssuers: []v1alpha1.ClusterIssuer{{
					Name: clusterIssuerName,
					Spec: clusterIssuer,
				}},
			}

			addonKey = types.NamespacedName{Name: addonName, Namespace: consts.NamespaceBoundlessSystem}
			issuerKey = types.NamespacedName{Name: issuerName, Namespace: issuerNamespace}
			clusterIssuerKey = types.NamespacedName{Name: clusterIssuerName, Namespace: consts.NamespaceBoundlessSystem}

		})
		Context("Helm chart addon is added to the blueprint", func() {
			BeforeEach(func() {
				By("Creating a blueprint with one addon")
				blueprint := newBlueprint(nil, helmAddon)
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

		Context("Helm chart addon is removed from blueprint", func() {
			It("Should delete addon resource", func() {
				By("Creating a blueprint with one addon")
				blueprint := newBlueprint(nil, helmAddon)
				Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())

				By("Waiting for addon to be created")
				actual := &v1alpha1.Addon{}
				Eventually(getObject(ctx, addonKey, actual), defaultTimeout, defaultInterval).Should(BeTrue())
				assertAddon(helmAddon, actual.Spec)

				By("Removing addon from blueprints")
				empty := newBlueprint(nil)
				Expect(createOrUpdateBlueprint(ctx, empty)).Should(Succeed())

				By("Checking if addon is removed")
				createdAddon := &v1alpha1.Addon{}
				Eventually(getObject(ctx, addonKey, createdAddon), defaultTimeout, defaultInterval).Should(BeFalse())
			})
		})

		Context("Issuer and ClusterIssuer are handled properly", func() {
			BeforeEach(func() {
				// This test case is skipped for the same reasons we skip controllers/installation_controller_test.go
				// See comments there for details.
				// Once the webhook can be installed as part of these tests, this test-case can be unskipped
				Skip("Skip issuer relates tests")

				By("Creating a blueprint with one issuer and one cluster issuer")
				blueprint := newBlueprint(&caSpec)
				Expect(createOrUpdateBlueprint(ctx, blueprint)).Should(Succeed())
			})

			It("Should create Issuer and ClusterIssuer successfully", func() {
				By("Checking if Issuer is created")
				createdIssuer := &certmanager.Issuer{}
				Eventually(getObject(ctx, issuerKey, createdIssuer)).Should(BeTrue())
				assertIssuer(issuer, createdIssuer.Spec)

				By("Checking if ClusterIssuer is created")
				createdClusterIssuer := &certmanager.ClusterIssuer{}
				Eventually(getObject(ctx, clusterIssuerKey, createdClusterIssuer)).Should(BeTrue())
				assertIssuer(clusterIssuer, createdClusterIssuer.Spec)
			})

			It("Should update Issuer and ClusterIssuer successfully", func() {
				By("Updating the blueprint with new issuer and cluster issuer")
				newIssuerName := randomName("new-issuer")
				newIssuerNamespace := randomName("new-ns")
				newClusterIssuerName := randomName("new-clusterissuer")

				newIssuer := certmanager.IssuerSpec{IssuerConfig: certmanager.IssuerConfig{
					CA: &certmanager.CAIssuer{
						SecretName: "new-ca-secret",
					},
				}}

				newClusterIssuer := certmanager.IssuerSpec{IssuerConfig: certmanager.IssuerConfig{
					CA: &certmanager.CAIssuer{
						SecretName: "new-cluster-ca-secret",
					},
				}}

				newCaSpec := v1alpha1.CASpec{
					Issuers: []v1alpha1.Issuer{{
						Name:      newIssuerName,
						Namespace: newIssuerNamespace,
						Spec:      newIssuer,
					}},
					ClusterIssuers: []v1alpha1.ClusterIssuer{{
						Name: newClusterIssuerName,
						Spec: newClusterIssuer,
					}},
				}

				updatedBlueprint := newBlueprint(&newCaSpec)
				Expect(createOrUpdateBlueprint(ctx, updatedBlueprint)).Should(Succeed())

				By("Checking if Issuer is updated")
				updatedIssuer := &certmanager.Issuer{}
				Eventually(getObject(ctx, types.NamespacedName{Name: newIssuerName, Namespace: newIssuerNamespace}, updatedIssuer)).Should(BeTrue())
				assertIssuer(newIssuer, updatedIssuer.Spec)

				By("Checking if ClusterIssuer is updated")
				updatedClusterIssuer := &certmanager.ClusterIssuer{}
				Eventually(getObject(ctx, types.NamespacedName{Name: newClusterIssuerName, Namespace: consts.NamespaceBoundlessSystem}, updatedClusterIssuer)).Should(BeTrue())
				assertIssuer(newClusterIssuer, updatedClusterIssuer.Spec)
			})

			It("Should delete Issuer and ClusterIssuer successfully", func() {
				By("Deleting the blueprint")
				empty := newBlueprint(nil)
				Expect(createOrUpdateBlueprint(ctx, empty)).Should(Succeed())

				By("Checking if Issuer is deleted")
				deletedIssuer := &certmanager.Issuer{}
				Eventually(getObject(ctx, issuerKey, deletedIssuer)).Should(BeFalse())

				By("Checking if ClusterIssuer is deleted")
				deletedClusterIssuer := &certmanager.ClusterIssuer{}
				Eventually(getObject(ctx, clusterIssuerKey, deletedClusterIssuer)).Should(BeFalse())
			})
		})
	})
})
