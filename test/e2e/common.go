package e2e

import (
	"context"

	"testing"
	"time"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/blueprint-operator/client/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

func newAddon(a metav1.ObjectMeta) *v1alpha1.Addon {
	return &v1alpha1.Addon{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Addon",
			APIVersion: "blueprint.mirantis.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
	}
}

func newIssuer(i metav1.ObjectMeta) *certmanager.Issuer {
	return &certmanager.Issuer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Issuer",
			APIVersion: "cert-manager.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.Name,
			Namespace: i.Namespace,
			Labels:    i.Labels,
		},
	}
}

func newClusterIssuer(ci metav1.ObjectMeta) *certmanager.ClusterIssuer {
	return &certmanager.ClusterIssuer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterIssuer",
			APIVersion: "cert-manager.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ci.Name,
		},
	}
}

func newCertificate(cert metav1.ObjectMeta) *certmanager.Certificate {
	return &certmanager.Certificate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Certificate",
			APIVersion: "cert-manager.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cert.Name,
			Namespace: cert.Namespace,
			Labels:    cert.Labels,
		},
	}
}

// ApplyCleanupBlueprint applies a blueprint with no addons to the cluster
// This is used to clean up the cluster after the tests
func ApplyCleanupBlueprint() features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dep := &v1alpha1.Blueprint{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Blueprint",
				APIVersion: "blueprint.mirantis.com/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blueprint-cluster",
				Namespace: consts.NamespaceBlueprintSystem,
			},
			Spec: v1alpha1.BlueprintSpec{
				Resources: v1alpha1.Resources{},
				Components: v1alpha1.Component{
					Addons: []v1alpha1.AddonSpec{},
				},
			},
		}

		existing := dep.DeepCopy()
		if err := c.Client().Resources().Get(ctx, dep.Name, dep.Namespace, existing); err != nil {
			t.Fatalf("failed to get blueprint: %v", err)
		}

		dep.SetFinalizers(existing.GetFinalizers())
		dep.SetResourceVersion(existing.GetResourceVersion())
		if err := c.Client().Resources().Update(ctx, dep); err != nil {
			t.Fatalf("failed to cleanup blueprint after test: %v", err)
		}
		return ctx
	}
}

func AssessCertificate(d time.Duration, cert *certmanager.Certificate, desiredSpecs certmanager.CertificateSpec) features.Func {
	return funcs.AllOf(
		funcs.CertificateHaveStatusWithin(d/2, cert, certmanagermeta.ConditionTrue),
		funcs.ResourceMatchWithin(d/2, cert, func(object k8s.Object) bool {
			c := object.(*certmanager.Certificate)
			return c.Spec.CommonName == desiredSpecs.CommonName &&
				c.Spec.IsCA == desiredSpecs.IsCA &&
				c.Spec.SecretName == desiredSpecs.SecretName &&
				c.Spec.IssuerRef.Name == desiredSpecs.IssuerRef.Name &&
				c.Spec.IssuerRef.Kind == desiredSpecs.IssuerRef.Kind
		}),
	)
}
