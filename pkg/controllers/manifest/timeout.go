package manifest

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	blueprintv1alpha1 "github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
)

// AwaitTimeout waits timeout duration and then checks the status of manifest denoted by provided manifestName
// If the manifest is not Available after timeout, AwaitTimeout returns an error
func (mc *Controller) AwaitTimeout(logger logr.Logger, manifestName types.NamespacedName, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, mc.checkManifestAvailableFunc(logger, manifestName))
}

func (mc *Controller) checkManifestAvailableFunc(logger logr.Logger, manifestName types.NamespacedName) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		var manifest blueprintv1alpha1.Manifest
		err := mc.client.Get(ctx, manifestName, &manifest)
		if err != nil {
			logger.Error(err, "failed to check on manifest")
			return false, fmt.Errorf("failed to get manifest : %w", err)
		}

		if manifest.Status.Type == blueprintv1alpha1.TypeComponentAvailable {
			logger.Info("manifest available before timeout", "manifestName", manifestName)
			return true, nil
		}

		return false, nil
	}
}

// ShouldRetryManifest checks if given manifest should be Retried
// Return false if the failurePolicy is not set to Retry or if the manifest is not considered unhealthy ( so Available or Progressing)
// Then check the CRD metadata for the last update made to this object (discount status updates)
// If the time elapsed since the last update is longer than the specified timeout, return true
func ShouldRetryManifest(logger logr.Logger, manifest *blueprintv1alpha1.Manifest) bool {
	if manifest.Spec.FailurePolicy != FailurePolicyRetry {
		return false
	}

	if manifest.Status.Type != blueprintv1alpha1.TypeComponentUnhealthy {
		return false
	}

	// manifest has policy "Retry" and is in state "Unhealthy"

	// if timeout is not specified then we should not retry
	if manifest.Spec.Timeout == "" {
		logger.Info("not retrying manifest: timeout not specified")
		return false
	}

	// if timeout is specified then we shouldn't retry the Manifest unless the time given for timeout has elapsed
	timeoutTime, err := time.ParseDuration(manifest.Spec.Timeout)
	if err != nil {
		// if timeout cannot be parsed treat it as if no timeout is specified
		logger.Error(err, "retry manifest could not parse timeout duration")
		return false
	}

	lastUpdateTime, err := getLatestUpdateTime(manifest.ObjectMeta.ManagedFields)
	if err != nil {
		return false
	}
	// check whether this manifest is still within the timeout
	logger.Info("retrying manifest : check timeout", "lastUpdateTime", lastUpdateTime, "timeout", timeoutTime, "now", time.Now())
	if time.Now().Before(lastUpdateTime.Add(timeoutTime)) {
		logger.Info("retrying manifest : not time to retry yet ")
		return false
	}

	return true
}

// getLatestUpdateTime gets the last time that the manifest was updated , ignoring status updates
func getLatestUpdateTime(managedFields []v1.ManagedFieldsEntry) (v1.Time, error) {

	for i := len(managedFields) - 1; i >= 0; i-- {
		managedField := managedFields[i]
		if managedField.Operation == v1.ManagedFieldsOperationUpdate && managedField.Subresource == "Status" {
			continue
		}

		return *managedField.Time, nil
	}

	return v1.Now(), fmt.Errorf("unable to get latest update time")
}
