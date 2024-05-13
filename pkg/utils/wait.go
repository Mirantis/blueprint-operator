package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	pollInterval = 5 * time.Second
	timeout      = 5 * time.Minute
)

// WaitForDeploymentReady waits for a deployment to be ready by checking:
// * if the available replicas match the desired replicas
// * if the deployment has the Available condition set to true
func WaitForDeploymentReady(ctx context.Context, log logr.Logger, runtimeClient client.Client, name, namespace string) error {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	return wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(context.Context) (bool, error) {
		dep := &appsv1.Deployment{}
		if err := runtimeClient.Get(ctx, key, dep); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		availableCondition, err := getConditionOfType(appsv1.DeploymentAvailable, dep.Status.Conditions)
		if err != nil {
			return false, nil
		}

		if dep.Status.AvailableReplicas == dep.Status.Replicas && availableCondition.Status == corev1.ConditionTrue {
			return true, nil
		}
		log.V(1).Info(fmt.Sprintf("waiting for deployment %s to %d replicas, currently at %d", name, dep.Status.Replicas, dep.Status.AvailableReplicas))
		return false, nil
	})
}

func getConditionOfType(desiredType appsv1.DeploymentConditionType, conditions []appsv1.DeploymentCondition) (appsv1.DeploymentCondition, error) {
	for _, condition := range conditions {
		if condition.Type == desiredType {
			return condition, nil
		}
	}

	return appsv1.DeploymentCondition{}, fmt.Errorf("condition type unavailable")
}
