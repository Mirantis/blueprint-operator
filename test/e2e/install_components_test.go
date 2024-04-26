package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

var curDir, _ = os.Getwd()

// TestInstallComponents tests the installation of two issuers, one cluster issuer, and two addons,
// one Helm addon and one Manifest addon,
// It checks if the issuers and addons are created, installed and their objects are created
func TestInstallComponents(t *testing.T) {
	dir := filepath.Join(curDir, "manifests")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: consts.NamespaceBoundlessSystem}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: consts.NamespaceBoundlessSystem}

	i1 := metav1.ObjectMeta{Name: "test-issuer-1", Namespace: "test-issuer-ns-1"}
	i2 := metav1.ObjectMeta{Name: "test-issuer-2", Namespace: "test-issuer-ns-1"}

	ci1 := metav1.ObjectMeta{Name: "test-cluster-issuer-1", Namespace: consts.NamespaceBoundlessSystem}

	a1dep := metav1.ObjectMeta{Name: "nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}

	f := features.New("InstallComponents").
		WithSetup("CreateBlueprint", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),
			funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),
		)).
		Assess("TwoAddonsAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a1)),
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a2)),
		)).
		Assess("TwoIssuersAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newIssuer(i2)),
		)).
		Assess("TwoClusterIssuersAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newClusterIssuer(ci1)),
		)).
		Assess("HelmAddonsIsSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
		)).
		Assess("ManifestAddonIsSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a2), v1alpha1.TypeComponentAvailable),
		)).
		Assess("HelmAddonObjectsAreSuccessfullyCreated", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
		)).
		Assess("ManifestAddonObjectsAreSuccessfullyCreated", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a2dep.Namespace, a2dep.Name),
		)).
		Assess("IssuerObjectsAreSuccessfullyCreated", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(DefaultWaitTimeout, newIssuer(i1), certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(DefaultWaitTimeout, newIssuer(i2), certmanagermeta.ConditionTrue),
		)).
		Assess("ClusterIssuerObjectsAreSuccessfullyCreated", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(DefaultWaitTimeout, newClusterIssuer(ci1), certmanagermeta.ConditionTrue),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a2)),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i2)),
			funcs.ResourceDeletedWithin(2*time.Minute, newClusterIssuer(ci1)),
		)).
		Feature()

	testenv.Test(t, f)
}
