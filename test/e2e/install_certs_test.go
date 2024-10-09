package e2e

import (
	"path/filepath"
	"testing"
	"time"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

// TestInstallCerts tests the installation of two issuers and one cluster issuer
// and two certificates, one for each issuer.
// It checks if all objects are created, installed and their objects are created
func TestInstallCerts(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "certs")

	i1 := newIssuer(metav1.ObjectMeta{
		Name:      "test-issuer-1",
		Namespace: "test-issuer-ns-1",
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "blueprint-operator",
		},
	})
	i2 := newIssuer(metav1.ObjectMeta{
		Name:      "test-issuer-2",
		Namespace: "test-issuer-ns-1",
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "blueprint-operator",
		},
	})

	ci1 := newClusterIssuer(metav1.ObjectMeta{Name: "test-cluster-issuer-1"})

	cert1 := newCertificate(metav1.ObjectMeta{
		Name:      "test-cert-1",
		Namespace: "test-issuer-ns-1",
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "blueprint-operator",
		},
	})
	cert1Specs := certmanager.CertificateSpec{
		CommonName: "test-cert-1",
		IsCA:       true,
		SecretName: "test-cert-secret-1",
		IssuerRef: certmanagermeta.ObjectReference{
			Name: i1.Name,
			Kind: "Issuer",
		},
	}
	cert2 := newCertificate(metav1.ObjectMeta{
		Name:      "test-cert-2",
		Namespace: "test-cert-ns-1",
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "blueprint-operator",
		},
	})
	cert2Specs := certmanager.CertificateSpec{
		CommonName: "test-cert-2",
		IsCA:       false,
		SecretName: "test-cert-secret-2",
		IssuerRef: certmanagermeta.ObjectReference{
			Name: ci1.Name,
			Kind: "ClusterIssuer",
		},
	}

	f := features.New("InstallIssuersAndCertificates").
		WithSetup("CreateBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBlueprintSystem, "cert-manager"),
		)).
		Assess("TwoIssuersAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, i1),
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, i2),
		)).
		Assess("TwoClusterIssuersAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, ci1),
		)).
		Assess("IssuerObjectsAreSuccessfullyCreated", funcs.AllOf(
			funcs.IssuerHaveStatusWithin(DefaultWaitTimeout, i1, certmanagermeta.ConditionTrue),
			funcs.IssuerHaveStatusWithin(DefaultWaitTimeout, i2, certmanagermeta.ConditionTrue),
		)).
		Assess("ClusterIssuerObjectsAreSuccessfullyCreated", funcs.AllOf(
			funcs.ClusterIssuerHaveStatusWithin(DefaultWaitTimeout, ci1, certmanagermeta.ConditionTrue),
		)).
		Assess("CertificatesAreSuccessfullyCreated", funcs.AllOf(
			AssessCertificate(2*time.Minute, cert1, cert1Specs),
			AssessCertificate(2*time.Minute, cert2, cert2Specs),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, i1),
			funcs.ResourceDeletedWithin(2*time.Minute, i2),
			funcs.ResourceDeletedWithin(2*time.Minute, ci1),
			funcs.ResourceDeletedWithin(2*time.Minute, cert1),
			funcs.ResourceDeletedWithin(2*time.Minute, cert2),
		)).
		Feature()

	testenv.Test(t, f)
}
