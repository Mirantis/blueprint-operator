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

// TestInstallIssuers tests the installation of two issuers and one cluster issuer
// It checks if the issuers are created, installed and their objects are created
func TestInstallIssuers(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "issuers")

	i1 := metav1.ObjectMeta{Name: "test-issuer-1", Namespace: "test-issuer-ns-1"}
	i2 := metav1.ObjectMeta{Name: "test-issuer-2", Namespace: "test-issuer-ns-1"}

	ci1 := metav1.ObjectMeta{Name: "test-cluster-issuer-1"}

	f := features.New("InstallIssuers").
		WithSetup("CreateBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),
		)).
		Assess("TwoIssuersAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newIssuer(i1)),
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newIssuer(i2)),
		)).
		Assess("TwoClusterIssuersAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newClusterIssuer(ci1)),
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
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i2)),
			funcs.ResourceDeletedWithin(2*time.Minute, newClusterIssuer(ci1)),
		)).
		Feature()

	testenv.Test(t, f)
}
