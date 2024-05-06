package e2e

import (
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"path/filepath"
	"testing"
	"time"

	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

// TestUninstallCerts tests the uninstallation of issuers, cluster issuers, and certificates:
//  1. Apply a blueprint with 3 issuers, 2 cluster issuers and 4 certificates
//  2. Uninstall 2 issuers, 1 cluster issuers, and 2 certificates
//     by applying a blueprint with 1 issuer, 1 cluster issuer, and 2 certificates
//  3. Ensure the issuer and certificate objects are removed
func TestUninstallCerts(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "certs")

	i1 := metav1.ObjectMeta{Name: "test-issuer-1", Namespace: "test-issuer-ns-1"}
	i2 := metav1.ObjectMeta{Name: "test-issuer-2", Namespace: "test-issuer-ns-2"}
	i3 := metav1.ObjectMeta{Name: "test-issuer-3", Namespace: "test-issuer-ns-1"}

	ci1 := metav1.ObjectMeta{Name: "test-cluster-issuer-1"}
	ci2 := metav1.ObjectMeta{Name: "test-cluster-issuer-2"}

	cert1 := newCertificate(metav1.ObjectMeta{Name: "test-cert-1", Namespace: "test-cert-ns-1"})
	cert2 := newCertificate(metav1.ObjectMeta{Name: "test-cert-2", Namespace: "test-cert-ns-2"})
	cert3 := newCertificate(metav1.ObjectMeta{Name: "test-cert-3", Namespace: "test-issuer-ns-2"})
	cert4 := newCertificate(metav1.ObjectMeta{Name: "test-cert-4", Namespace: "test-cert-ns-1"})

	certSpecs := []certmanager.CertificateSpec{
		{
			CommonName: "test-cert-1",
			IsCA:       true,
			SecretName: "test-cert-secret-11",
			IssuerRef: certmanagermeta.ObjectReference{
				Name: ci2.Name,
				Kind: "ClusterIssuer",
			},
		},
		{
			CommonName: "test-cert-3",
			IsCA:       false,
			SecretName: "test-cert-secret-3",
			IssuerRef: certmanagermeta.ObjectReference{
				Name: i2.Name,
				Kind: "Issuer",
			},
		},
	}

	f := features.New("Uninstall Issuers And Certs").
		WithSetup("CreatePrerequisiteIssuers", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),

			// ensure all components are installed and available before we start deleting them
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i1), certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i2), certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(2*time.Minute, newIssuer(i3), certmanagermeta.ConditionTrue),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci1), certmanagermeta.ConditionFalse),
			funcs.ClusterIssuerHaveStatusWithin(2*time.Minute, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
			funcs.CertificateHaveStatusWithin(2*time.Minute, cert1, certmanagermeta.ConditionTrue),
			funcs.CertificateHaveStatusWithin(2*time.Minute, cert2, certmanagermeta.ConditionTrue),
			funcs.CertificateHaveStatusWithin(2*time.Minute, cert3, certmanagermeta.ConditionTrue),
			funcs.CertificateHaveStatusWithin(2*time.Minute, cert4, certmanagermeta.ConditionTrue),
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
		Assess("AllRemovedCertificatesHaveBeenDeleted", funcs.AllOf(
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, cert2),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, cert4),
		)).
		Assess("Issuer2StillAvailable", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(DefaultWaitTimeout, newIssuer(i2), certmanagermeta.ConditionTrue),
		)).
		Assess("ClusterIssuer2StillAvailable", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(DefaultWaitTimeout, newClusterIssuer(ci2), certmanagermeta.ConditionTrue),
		)).
		Assess("CertificatesStillAvailable", funcs.AllOf(
			AssessCertificate(2*time.Minute, cert1, certSpecs[0]),
			AssessCertificate(2*time.Minute, cert3, certSpecs[1]),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newIssuer(i1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newClusterIssuer(ci2)),
			funcs.ResourceDeletedWithin(2*time.Minute, cert1),
			funcs.ResourceDeletedWithin(2*time.Minute, cert3),
		)).
		Feature()

	testenv.Test(t, f)
}
