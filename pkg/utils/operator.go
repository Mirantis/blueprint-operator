package utils

import (
	"context"
	"fmt"

	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
)

// GetOperatorImage returns the image used by the operator.
func GetOperatorImage(ctx context.Context, k8sClient client.Client) (string, error) {
	key := client.ObjectKey{
		Namespace: consts.NamespaceBoundlessSystem,
		Name:      consts.BoundlessOperatorName,
	}

	d := &v1.Deployment{}
	if err := k8sClient.Get(ctx, key, d); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("operator deployment %s/%s not found", consts.NamespaceBoundlessSystem, consts.BoundlessOperatorName)
		}
		return "", fmt.Errorf("failed to get operator deployment: %w", err)
	}

	for _, container := range d.Spec.Template.Spec.Containers {
		if container.Name == consts.BoundlessContainerName {
			return container.Image, nil
		}
	}

	return "", fmt.Errorf("operator container not found in deployment %s/%s", consts.NamespaceBoundlessSystem, consts.BoundlessOperatorName)
}
