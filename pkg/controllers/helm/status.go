package helm

import (
	"slices"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobStatus int

const (
	ReleaseStatusSuccess     string = "Success"
	ReleaseStatusFailed      string = "Failed"
	ReleaseStatusProgressing string = "Progressing"
)

// DetermineReleaseStatus determines the status of the release based on status fields
func DetermineReleaseStatus(release *helmv2.HelmRelease) string {

	failedReasons := []string{
		helmv2.InstallFailedReason,
		helmv2.UpgradeFailedReason,
		helmv2.RollbackFailedReason,
		helmv2.UninstallFailedReason,
	}

	// Check if the release has a "Released" condition
	// If the condition is true, the release was successful
	// If the condition is false and the reason is InstallFailed, the release failed
	// Otherwise, the release is still in progress
	for _, cond := range release.Status.Conditions {
		if cond.Type == helmv2.ReleasedCondition {
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
