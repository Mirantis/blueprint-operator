package helm

import (
	"github.com/fluxcd/helm-controller/api/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobStatus int

const (
	ReleaseStatusSuccess int = iota
	ReleaseStatusFailed
	ReleaseStatusProgressing
)

// DetermineReleaseStatus determines the status of the release based on status fields
func DetermineReleaseStatus(release *v2beta2.HelmRelease) int {

	cond := getReleasedCondition(release)
	if cond != nil {
		if cond.Status == metav1.ConditionTrue {
			return ReleaseStatusSuccess
		}
		if cond.Status == metav1.ConditionFalse && cond.Reason == v2beta2.InstallFailedReason {
			return ReleaseStatusFailed
		}
	}
	return ReleaseStatusProgressing
}

func getReleasedCondition(release *v2beta2.HelmRelease) *metav1.Condition {
	for i := range release.Status.Conditions {
		if release.Status.Conditions[i].Type == v2beta2.ReleasedCondition {
			return &release.Status.Conditions[i]
		}
	}
	return nil
}
