package helm

import (
	"slices"

	"github.com/fluxcd/helm-controller/api/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobStatus int

const (
	ReleaseStatusSuccess     string = "Success"
	ReleaseStatusFailed      string = "Failed"
	ReleaseStatusProgressing string = "Progressing"
)

// DetermineReleaseStatus determines the status of the release based on status fields
func DetermineReleaseStatus(release *v2beta2.HelmRelease) string {

	failedReasons := []string{
		v2beta2.InstallFailedReason,
		v2beta2.UpgradeFailedReason,
		v2beta2.RollbackFailedReason,
		v2beta2.UninstallFailedReason,
	}

	// Check if the release has a "Released" condition
	// If the condition is true, the release was successful
	// If the condition is false and the reason is InstallFailed, the release failed
	// Otherwise, the release is still in progress
	for _, cond := range release.Status.Conditions {
		if cond.Type == v2beta2.ReleasedCondition {
			if cond.Status == metav1.ConditionTrue {
				return ReleaseStatusSuccess
			}
			if cond.Status == metav1.ConditionFalse && slices.Contains(failedReasons, cond.Reason) {
				return ReleaseStatusFailed
			}
		}
	}

	return ReleaseStatusProgressing
}
