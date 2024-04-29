package e2e

import (
	"path/filepath"
	"testing"
	"time"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

// TestUpdateIssuers tests the update of issuers and cluster issuers in the cluster
//  1. Creates a blueprint with two issuers and one cluster issuer
//  2. Updates the blueprint to include one new issuer and one new cluster issuer
//     and updates the existing issuers
//  3. Checks that the existing issuers are updated to the new version
//  4. Checks that the new issuers are installed
func TestUpdateIssuers(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "issuers")

	i1 := metav1.ObjectMeta{Name: "test-issuer-1", Namespace: "test-issuer-ns-1"}
	i2 := metav1.ObjectMeta{Name: "test-issuer-2", Namespace: "test-issuer-ns-1"}
	i3 := metav1.ObjectMeta{Name: "test-issuer-3", Namespace: "test-issuer-ns-1"}

	ci1 := metav1.ObjectMeta{Name: "test-cluster-issuer-1"}
	ci2 := metav1.ObjectMeta{Name: "test-cluster-issuer-2"}

	f := features.New("Update Issuers").
		WithSetup("CreatePrerequisiteBlueprint", funcs.AllOf(
			// create the blueprint with two addons, issuer, and cluster issuer, that will be updated later
			funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),

			// wait for the components to be installed
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i1), certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i2), certmanagermeta.ConditionTrue),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci1), certmanagermeta.ConditionTrue),
		)).
		WithSetup("UpdateBlueprint", funcs.AllOf(
			// update the blueprint to include two new addons and update the existing ones
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),
		)).
		Assess("ExistingIssuersStillExists", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newIssuer(i2)),
		)).
		Assess("ExistingClusterIssuersStillExists", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(2*time.Minute, newClusterIssuer(ci1)),
		)).
		Assess("ExistingIssuerAreSuccessfullyInstalled", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i1), certmanagermeta.ConditionFalse),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i2), certmanagermeta.ConditionTrue),
		)).
		Assess("ExistingClusterIssuerIsSuccessfullyInstalled", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci1), certmanagermeta.ConditionFalse),
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
		Assess("NewIssuerIsCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newIssuer(i3)),
		)).
		Assess("NewClusterIssuerIsCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newClusterIssuer(ci2)),
		)).
		Assess("NewIssuerIsSuccessfullyInstalled", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i3), certmanagermeta.ConditionTrue),
		)).
		Assess("NewClusterIssuerIsSuccessfullyInstalled", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i2)),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i3)),
			funcs.ResourceDeletedWithin(2*time.Minute, newClusterIssuer(ci1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newClusterIssuer(ci2)),
		)).
		Feature()

	testenv.Test(t, f)
}
