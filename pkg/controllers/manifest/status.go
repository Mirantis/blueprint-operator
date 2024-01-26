package manifest

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	apps_v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Status struct {
	StatusType boundlessv1alpha1.StatusType
	Reason     string
	Message    string
}

// CheckManifestStatus checks the status of any deployments and daemonsets associated with the namespacedName manifest
// Check the status of the deployment and daemonset and set the manifest to an error state if any errors are found
// If no errors are found, we check if any deployments/daemonsets are still progressing and set the manifest status to Progressing
// Otherwise set the manifest status to Available
// This is not comprehensive and may need to be updated as we support more complex manifests
func (mc *ManifestController) CheckManifestStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, objects []boundlessv1alpha1.ManifestObject) (Status, error) {

	if objects == nil || len(objects) == 0 {
		logger.Info("No manifest objects for manifest")
		return Status{boundlessv1alpha1.TypeComponentUnhealthy, "No objects detected for manifest", ""}, nil
	}

	// split objects into deployments and daemonsets, which are the two resources we are looking at for status currently since
	// they have reliable status fields
	var deployments, daemonsets []boundlessv1alpha1.ManifestObject
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

	if deploymentStatus.StatusType == boundlessv1alpha1.TypeComponentUnhealthy {
		return deploymentStatus, nil
	}

	if daemonsetStatus.StatusType == boundlessv1alpha1.TypeComponentUnhealthy {
		return daemonsetStatus, nil
	}

	if deploymentStatus.StatusType == boundlessv1alpha1.TypeComponentProgressing && daemonsetStatus.StatusType == boundlessv1alpha1.TypeComponentProgressing {
		return Status{boundlessv1alpha1.TypeComponentAvailable, "Manifest Components Still Progressing", fmt.Sprintf("Deployments : %s, Daemonsets : %s", deploymentStatus.Reason, daemonsetStatus.Reason)}, nil
	} else if deploymentStatus.StatusType == boundlessv1alpha1.TypeComponentProgressing {
		return deploymentStatus, nil
	} else if daemonsetStatus.StatusType == boundlessv1alpha1.TypeComponentProgressing {
		return daemonsetStatus, nil
	}

	// if we got here then both deployments and daemonsets are Available

	return Status{boundlessv1alpha1.TypeComponentAvailable, "Manifest Components Available", fmt.Sprintf("Deployments : %s, Daemonsets : %s", deploymentStatus.Reason, daemonsetStatus.Reason)}, nil
}

func (mc *ManifestController) checkManifestDeployments(ctx context.Context, logger logr.Logger, deployments []boundlessv1alpha1.ManifestObject) (Status, error) {

	if len(deployments) == 0 {
		return Status{}, nil
	}

	progressCount := 0

	for _, obj := range deployments {

		deployment := &apps_v1.Deployment{}
		err := mc.client.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}, deployment)
		if err != nil {
			return Status{boundlessv1alpha1.TypeComponentUnhealthy, "Unable to get deployment from manifest", ""}, err

		}
		if deployment.Status.AvailableReplicas == deployment.Status.Replicas && (deployment.Status.Conditions == nil || len(deployment.Status.Conditions) == 0) {
			// this deployment is ready
			continue
		}
		latestCondition := deployment.Status.Conditions[0]
		if deployment.Status.AvailableReplicas == deployment.Status.Replicas && latestCondition.Type == apps_v1.DeploymentAvailable {
			// this deployment is ready
			continue
		}

		if latestCondition.Type == apps_v1.DeploymentProgressing || latestCondition.Reason == "MinimumReplicasUnavailable" {
			progressCount++
		} else {
			// deployment is in error state, so we can return error status for deployments
			if err != nil {
				return Status{boundlessv1alpha1.TypeComponentUnhealthy, latestCondition.Reason, latestCondition.Message}, err
			}
		}
	}

	if progressCount > 0 {
		// if even 1 deployment is still progressing we should return progressing status
		return Status{boundlessv1alpha1.TypeComponentProgressing, "1 or more manifest deployments are still progressing", ""}, nil
	}

	return Status{boundlessv1alpha1.TypeComponentAvailable, "Manifest Deployments Available", ""}, nil
}

func (mc *ManifestController) checkManifestDaemonsets(ctx context.Context, logger logr.Logger, daemonsets []boundlessv1alpha1.ManifestObject) (Status, error) {

	if len(daemonsets) == 0 {
		return Status{}, nil
	}

	progressCount := 0

	for _, obj := range daemonsets {

		daemonset := &apps_v1.DaemonSet{}
		err := mc.client.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}, daemonset)
		if err != nil {
			return Status{boundlessv1alpha1.TypeComponentUnhealthy, "Unable to get daemonset from manifest", ""}, err
		}

		if daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberReady && daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberAvailable {
			//daemonset is ready
			continue
		}

		if daemonset.Status.NumberMisscheduled > 0 || daemonset.Status.NumberUnavailable > 0 {
			if err != nil {
				return Status{boundlessv1alpha1.TypeComponentUnhealthy, fmt.Sprintf("Daemonset %s failed to schedule pods", daemonset.Name), ""}, err
			}
		}

		progressCount++
	}

	if progressCount > 0 {
		// if even 1 daemonset is still progressing we should return progressing status
		return Status{boundlessv1alpha1.TypeComponentProgressing, "1 or more manifest daemonsets are still progressing", ""}, nil
	}

	return Status{boundlessv1alpha1.TypeComponentAvailable, "Manifest Daemonsets Available", ""}, nil
}
