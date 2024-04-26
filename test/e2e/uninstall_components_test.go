package e2e

import (
	"path/filepath"
	"testing"
	"time"

	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

// TestUninstallComponents tests the uninstallation of issuers, cluster issuers, helm and manifest addons:
//  1. Apply a blueprint with 4 addons, 3 issuers, and 2 cluster issuers
//  2. Uninstall 3 addons, 2 issuers and 1 cluster issuers
//     by applying a blueprint with 1 addon, 1 issuer, and 1 cluster issuer
//  3. Ensure the 3 addons, issuers and their objects are removed
func TestUninstallComponents(t *testing.T) {
	dir := filepath.Join(curDir, "manifests")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: consts.NamespaceBoundlessSystem}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: consts.NamespaceBoundlessSystem}
	a3 := metav1.ObjectMeta{Name: "test-addon-3", Namespace: consts.NamespaceBoundlessSystem}

	i1 := metav1.ObjectMeta{Name: "test-issuer-1", Namespace: "test-issuer-ns-1"}
	i2 := metav1.ObjectMeta{Name: "test-issuer-2", Namespace: "test-issuer-ns-2"}
	i3 := metav1.ObjectMeta{Name: "test-issuer-3", Namespace: "test-issuer-ns-1"}

	ci1 := metav1.ObjectMeta{Name: "test-cluster-issuer-1", Namespace: consts.NamespaceBoundlessSystem}
	ci2 := metav1.ObjectMeta{Name: "test-cluster-issuer-2", Namespace: consts.NamespaceBoundlessSystem}

	a1dep := metav1.ObjectMeta{Name: "nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}
	a3dep := metav1.ObjectMeta{Name: "crossplane", Namespace: "default"}

	f := features.New("Uninstall Components").
		WithSetup("CreatePrerequisiteAddons", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),
			// apply a blueprint with 3 addons
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),

			// ensure all components are installed and available before we start deleting them
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a2), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a3), v1alpha1.TypeComponentAvailable),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i1), certmanagermeta.ConditionFalse),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i2), certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i3), certmanagermeta.ConditionTrue),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci1), certmanagermeta.ConditionFalse),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
		)).
		WithSetup("DeleteAddonsWithBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/delete.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/delete.yaml"),
		)).
		Assess("AllRemovedAddonsHaveBeenDeleted", funcs.AllOf(
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newAddon(a2)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newAddon(a3)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newIssuer(i1)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newIssuer(i3)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newClusterIssuer(ci1)),
		)).
		Assess("Addon2ObjectsHasBeenDeleted", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a2dep.Name, Namespace: a2dep.Namespace}}),
		)).
		Assess("Addon3ObjectsHasBeenDeleted", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a3dep.Name, Namespace: a3dep.Namespace}}),
		)).
		Assess("Addon1StillAvailable", funcs.AllOf(
			funcs.AddonHaveStatusWithin(DefaultWaitTimeout, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
		)).
		Assess("IssuerStillAvailable", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(DefaultWaitTimeout, newIssuer(i2), certmanagermeta.ConditionTrue),
		)).
		Assess("ClusterIssuerStillAvailable", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(DefaultWaitTimeout, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.DeleteResource(newAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
		)).
		Feature()

	testenv.Test(t, f)
}
