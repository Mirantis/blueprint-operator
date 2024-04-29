package e2e

import (
	"context"
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
)

func newAddon(a metav1.ObjectMeta) *v1alpha1.Addon {
	return &v1alpha1.Addon{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Addon",
			APIVersion: "boundless.mirantis.com/v1alpha1",
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

// ApplyCleanupBlueprint applies a blueprint with no addons to the cluster
// This is used to clean up the cluster after the tests
func ApplyCleanupBlueprint() features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dep := &v1alpha1.Blueprint{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Blueprint",
				APIVersion: "boundless.mirantis.com/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "boundless-cluster",
				Namespace: consts.NamespaceBoundlessSystem,
			},
			Spec: v1alpha1.BlueprintSpec{
				Components: v1alpha1.Component{
					Addons: []v1alpha1.AddonSpec{},
					CAs:    v1alpha1.CASpec{},
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
