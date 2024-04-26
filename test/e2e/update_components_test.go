package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

// TestUpdateComponents tests the update of Helm and Manifest addons, issuers, and cluster issuers in the cluster
//  1. Creates a blueprint with two addons, two issuers, and one cluster issuer
//  2. Updates the blueprint to include one new issuer, one new cluster issuer, two new addons
//     and updates the existing addons and issuers
//  3. Checks that the existing addons and issuers are updated to the new version
//  4. Checks that the new addons and issuers are installed
func TestUpdateComponents(t *testing.T) {
	dir := filepath.Join(curDir, "manifests")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: consts.NamespaceBoundlessSystem}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: consts.NamespaceBoundlessSystem}
	a3 := metav1.ObjectMeta{Name: "test-addon-3", Namespace: consts.NamespaceBoundlessSystem}

	i1 := metav1.ObjectMeta{Name: "test-issuer-1", Namespace: "test-issuer-ns-1"}
	i2 := metav1.ObjectMeta{Name: "test-issuer-2", Namespace: "test-issuer-ns-1"}
	i3 := metav1.ObjectMeta{Name: "test-issuer-3", Namespace: "test-issuer-ns-1"}

	ci1 := metav1.ObjectMeta{Name: "test-cluster-issuer-1", Namespace: consts.NamespaceBoundlessSystem}
	ci2 := metav1.ObjectMeta{Name: "test-cluster-issuer-2", Namespace: consts.NamespaceBoundlessSystem}

	a1dep := metav1.ObjectMeta{Name: "nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}
	a3dep := metav1.ObjectMeta{Name: "crossplane", Namespace: "default"}

	helmAddonUpdatedVersion := "15.9.1"
	manifestAddonUpdatedVersion := "v0.13.12"

	f := features.New("Update Addons").
		WithSetup("CreatePrerequisiteBlueprint", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),
			// create the blueprint with two addons, issuer, and cluster issuer, that will be updated later
			funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),

			// wait for the components to be installed
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a2), v1alpha1.TypeComponentAvailable),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i1), certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i2), certmanagermeta.ConditionTrue),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci1), certmanagermeta.ConditionTrue),
		)).
		WithSetup("UpdateBlueprint", funcs.AllOf(
			// update the blueprint to include two new addons and update the existing ones
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),
		)).
		Assess("ExistingAddonsStillExists", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a1)),
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a2)),
		)).
		Assess("ExistingIssuersStillExists", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newIssuer(i2)),
		)).
		Assess("ExistingClusterIssuersStillExists", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newClusterIssuer(ci1)),
		)).
		Assess("ExistingAddonsAreSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(DefaultWaitTimeout, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(DefaultWaitTimeout, newAddon(a2), v1alpha1.TypeComponentAvailable),
		)).
		Assess("ExistingIssuerAreSuccessfullyInstalled", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i1), certmanagermeta.ConditionFalse),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i2), certmanagermeta.ConditionTrue),
		)).
		Assess("ExistingClusterIssuerIsSuccessfullyInstalled", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci1), certmanagermeta.ConditionFalse),
		)).
		Assess("ExistingHelmAddonIsSuccessfullyUpdated", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
			funcs.ResourceMatchWithin(DefaultWaitTimeout, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a1dep.Name, Namespace: a1dep.Namespace}}, func(object k8s.Object) bool {
				// check that the helm label of the deployment has been updated
				o := object.(*appsv1.Deployment)
				actual := o.Labels["helm.sh/chart"]
				expected := fmt.Sprintf("nginx-%s", helmAddonUpdatedVersion)
				t.Logf("actual: %s, expected: %s", actual, expected)
				return actual == expected
			}),
		)).
		Assess("ExistingManifestAddonIsSuccessfullyUpdated", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a2dep.Namespace, a2dep.Name),
			funcs.ResourceMatchWithin(DefaultWaitTimeout, &appsv1.Deployment{ObjectMeta: a2dep}, func(object k8s.Object) bool {
				o := object.(*appsv1.Deployment)
				imageName := o.Spec.Template.Spec.Containers[0].Image
				return strings.Contains(imageName, manifestAddonUpdatedVersion)
			}),
		)).
		Assess("ExistingIssuersAreSuccessfullyUpdated", funcs.AllOf(
			funcs.ResourceMatchWithin(DefaultWaitTimeout, newIssuer(i1), func(object k8s.Object) bool {
				o := object.(*certmanager.Issuer)
				return o.Spec.SelfSigned == nil && o.Spec.CA != nil && o.Spec.CA.SecretName == "test-issuer-secret"
			}),
			funcs.ResourceMatchWithin(DefaultWaitTimeout, newIssuer(i2), func(object k8s.Object) bool {
				o := object.(*certmanager.Issuer)
				return o.Namespace == "test-issuer-ns-2"
			}),
		)).
		Assess("ExistingClusterIssuerIsSuccessfullyUpdated", funcs.AllOf(
			funcs.ResourceMatchWithin(DefaultWaitTimeout, newClusterIssuer(ci1), func(object k8s.Object) bool {
				o := object.(*certmanager.ClusterIssuer)
				return o.Spec.SelfSigned == nil && o.Spec.CA != nil && o.Spec.CA.SecretName == "test-cluster-issuer-secret"
			}),
		)).
		Assess("NewAddonsAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a3)),
		)).
		Assess("NewIssuerIsCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newIssuer(i3)),
		)).
		Assess("NewClusterIssuerIsCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newClusterIssuer(ci2)),
		)).
		Assess("NewAddonsAreSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a3), v1alpha1.TypeComponentAvailable),
		)).
		Assess("NewIssuerIsSuccessfullyInstalled", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i3), certmanagermeta.ConditionTrue),
		)).
		Assess("NewClusterIssuerIsSuccessfullyInstalled", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
		)).
		Assess("NewAddonObjectsSuccessfullyCreated", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a3dep.Namespace, a3dep.Name),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a2)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a3)),
		)).
		Feature()

	testenv.Test(t, f)
}
