package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	pollInterval = 5 * time.Second
	timeout      = 5 * time.Minute
)

// WaitForDeploymentReady waits for a deployment to be ready
func WaitForDeploymentReady(ctx context.Context, log logr.Logger, runtimeClient client.Client, name, namespace string) error {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	return wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(context.Context) (bool, error) {
		dep := &v1.Deployment{}
		if err := runtimeClient.Get(ctx, key, dep); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if dep.Status.AvailableReplicas == dep.Status.Replicas {
			// Expected replicas active
			return true, nil
		}
		log.V(1).Info(fmt.Sprintf("waiting for deployment %s to %d replicas, currently at %d", name, dep.Status.Replicas, dep.Status.AvailableReplicas))
		return false, nil
	})
}
