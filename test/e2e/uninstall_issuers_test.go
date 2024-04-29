package e2e

import (
	"path/filepath"
	"testing"
	"time"

	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

// TestUninstallIssuers tests the uninstallation of issuers, cluster issuers:
//  1. Apply a blueprint with 3 issuers, and 2 cluster issuers
//  2. Uninstall 2 issuers and 1 cluster issuers
//     by applying a blueprint with 1 issuer, and 1 cluster issuer
//  3. Ensure the issuers and their objects are removed
func TestUninstallIssuers(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "issuers")

	i1 := metav1.ObjectMeta{Name: "test-issuer-1", Namespace: "test-issuer-ns-1"}
	i2 := metav1.ObjectMeta{Name: "test-issuer-2", Namespace: "test-issuer-ns-2"}
	i3 := metav1.ObjectMeta{Name: "test-issuer-3", Namespace: "test-issuer-ns-1"}

	ci1 := metav1.ObjectMeta{Name: "test-cluster-issuer-1"}
	ci2 := metav1.ObjectMeta{Name: "test-cluster-issuer-2"}

	f := features.New("Uninstall Issuers").
		WithSetup("CreatePrerequisiteIssuers", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),

			// ensure all components are installed and available before we start deleting them
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i1), certmanagermeta.ConditionFalse),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i2), certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i3), certmanagermeta.ConditionTrue),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci1), certmanagermeta.ConditionFalse),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
		)).
		WithSetup("DeleteIssuersWithBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/delete.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/delete.yaml"),
		)).
		Assess("AllRemovedIssuersHaveBeenDeleted", funcs.AllOf(
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newIssuer(i1)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newIssuer(i3)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newClusterIssuer(ci1)),
		)).
		Assess("Issuer2StillAvailable", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(DefaultWaitTimeout, newIssuer(i2), certmanagermeta.ConditionTrue),
		)).
		Assess("ClusterIssuer2StillAvailable", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(DefaultWaitTimeout, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newClusterIssuer(ci2)),
		)).
		Feature()

	testenv.Test(t, f)
}
