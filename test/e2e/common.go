package e2e

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

func makeAddon(a metav1.ObjectMeta) *v1alpha1.Addon {
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
