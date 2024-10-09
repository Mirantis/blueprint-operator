package manifest

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	blueprintv1alpha1 "github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
)

type Status struct {
	StatusType blueprintv1alpha1.StatusType
	Reason     string
	Message    string
}

// CheckManifestStatus checks the status of any deployments and daemonsets associated with the namespacedName manifest
// Check the status of the deployment and daemonset and set the manifest to an error state if any errors are found
// If no errors are found, we check if any deployments/daemonsets are still progressing and set the manifest status to Progressing
// Otherwise set the manifest status to Available
// This is not comprehensive and may need to be updated as we support more complex manifests
func (mc *Controller) CheckManifestStatus(ctx context.Context, logger logr.Logger, objects []blueprintv1alpha1.ManifestObject) (Status, error) {

	if objects == nil || len(objects) == 0 {
		logger.Info("No manifest objects for manifest")
		return Status{blueprintv1alpha1.TypeComponentUnhealthy, "No objects detected for manifest", ""}, nil
	}

	// split objects into deployments and daemonsets, which are the two resources we are looking at for status currently since
	// they have reliable status fields
	var deployments, daemonsets []blueprintv1alpha1.ManifestObject
	for _, obj := range objects {
		if obj.Kind == "Deployment" {
			deployments = append(deployments, obj)
		} else if obj.Kind == "DaemonSet" {
			daemonsets = append(daemonsets, obj)
		}
	}

	deploymentStatus, err := mc.checkManifestDeployments(ctx, logger, deployments)
	if err != nil {
		return deploymentStatus, err
	}

	daemonsetStatus, err := mc.checkManifestDaemonsets(ctx, logger, daemonsets)
	if err != nil {
		return daemonsetStatus, err
	}

	if deploymentStatus.StatusType == blueprintv1alpha1.TypeComponentUnhealthy {
		return deploymentStatus, nil
	}

	if daemonsetStatus.StatusType == blueprintv1alpha1.TypeComponentUnhealthy {
		return daemonsetStatus, nil
	}

	if deploymentStatus.StatusType == blueprintv1alpha1.TypeComponentProgressing && daemonsetStatus.StatusType == blueprintv1alpha1.TypeComponentProgressing {
		return Status{blueprintv1alpha1.TypeComponentProgressing, "Manifest Components Still Progressing", fmt.Sprintf("Deployments : %s, Daemonsets : %s", deploymentStatus.Reason, daemonsetStatus.Reason)}, nil
	} else if deploymentStatus.StatusType == blueprintv1alpha1.TypeComponentProgressing {
		return deploymentStatus, nil
	} else if daemonsetStatus.StatusType == blueprintv1alpha1.TypeComponentProgressing {
		return daemonsetStatus, nil
	}

	// if we got here then both deployments and daemonsets are Available

	return Status{blueprintv1alpha1.TypeComponentAvailable, "Manifest Components Available", fmt.Sprintf("Deployments : %s, Daemonsets : %s", deploymentStatus.Reason, daemonsetStatus.Reason)}, nil
}

func (mc *Controller) checkManifestDeployments(ctx context.Context, logger logr.Logger, deployments []blueprintv1alpha1.ManifestObject) (Status, error) {

	if len(deployments) == 0 {
		return Status{}, nil
	}

	progressCount := 0

	for _, obj := range deployments {

		deployment := &appsv1.Deployment{}
		err := mc.client.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}, deployment)
		if err != nil {
			return Status{blueprintv1alpha1.TypeComponentUnhealthy, "Unable to get deployment from manifest", ""}, err
		}
		if deployment.Status.AvailableReplicas == deployment.Status.Replicas && (deployment.Status.Conditions == nil || len(deployment.Status.Conditions) == 0) {
			// this deployment is ready
			continue
		}

		progressCondition, err := getConditionOfType(appsv1.DeploymentProgressing, deployment.Status.Conditions)
		if err != nil {
			return Status{blueprintv1alpha1.TypeComponentUnhealthy, "Unable to get deployment conditions from manifest", ""}, err
		}

		availableCondition, err := getConditionOfType(appsv1.DeploymentAvailable, deployment.Status.Conditions)
		if err != nil {
			return Status{blueprintv1alpha1.TypeComponentUnhealthy, "Unable to get deployment conditions from manifest", ""}, err
		}

		if deployment.Status.AvailableReplicas == deployment.Status.Replicas && availableCondition.Status == v1.ConditionTrue {
			// this deployment is ready
			continue
		}

		// if progress condition is not true, then progress deadline has not yet expired for the deployment
		if progressCondition.Status == v1.ConditionTrue {
			progressCount++
		} else {
			// progress deadline has expired for deployment, so we can return error status for deployments
			return Status{blueprintv1alpha1.TypeComponentUnhealthy, progressCondition.Reason, progressCondition.Message}, nil

		}
	}

	if progressCount > 0 {
		// if even 1 deployment is still progressing we should return progressing status
		return Status{blueprintv1alpha1.TypeComponentProgressing, "1 or more manifest deployments are still progressing", ""}, nil
	}

	return Status{blueprintv1alpha1.TypeComponentAvailable, "Manifest Deployments Available", ""}, nil
}

func getConditionOfType(desiredType appsv1.DeploymentConditionType, conditions []appsv1.DeploymentCondition) (appsv1.DeploymentCondition, error) {
	for _, condition := range conditions {
		if condition.Type == desiredType {
			return condition, nil
		}
	}

	return appsv1.DeploymentCondition{}, fmt.Errorf("condition type unavailable")
}

func (mc *Controller) checkManifestDaemonsets(ctx context.Context, logger logr.Logger, daemonsets []blueprintv1alpha1.ManifestObject) (Status, error) {

	if len(daemonsets) == 0 {
		return Status{}, nil
	}

	progressCount := 0

	for _, obj := range daemonsets {

		daemonset := &appsv1.DaemonSet{}
		err := mc.client.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}, daemonset)
		if err != nil {
			return Status{blueprintv1alpha1.TypeComponentUnhealthy, "Unable to get daemonset from manifest", ""}, err
		}

		if daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberReady && daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberAvailable {
			//daemonset is ready
			continue
		}

		if daemonset.Status.NumberMisscheduled > 0 || daemonset.Status.NumberUnavailable > 0 {
			if err != nil {
				return Status{blueprintv1alpha1.TypeComponentUnhealthy, fmt.Sprintf("Daemonset %s failed to schedule pods", daemonset.Name), ""}, err
			}
		}

		progressCount++
	}

	if progressCount > 0 {
		// if even 1 daemonset is still progressing we should return progressing status
		return Status{blueprintv1alpha1.TypeComponentProgressing, "1 or more manifest daemonsets are still progressing", ""}, nil
	}

	return Status{blueprintv1alpha1.TypeComponentAvailable, "Manifest Daemonsets Available", ""}, nil
}
