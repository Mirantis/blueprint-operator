package helm

import (
	"testing"

	helmapiv2 "github.com/fluxcd/helm-controller/api/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetermineReleaseStatus(t *testing.T) {
	tests := []struct {
		name     string
		release  *helmapiv2.HelmRelease
		expected string
	}{
		{
			name: "ReleaseStatusSuccess",
			release: &helmapiv2.HelmRelease{
				Status: helmapiv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmapiv2.ReleasedCondition,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: ReleaseStatusSuccess,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmapiv2.HelmRelease{
				Status: helmapiv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmapiv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmapiv2.InstallFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmapiv2.HelmRelease{
				Status: helmapiv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmapiv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmapiv2.UpgradeFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmapiv2.HelmRelease{
				Status: helmapiv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmapiv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmapiv2.RollbackFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmapiv2.HelmRelease{
				Status: helmapiv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmapiv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmapiv2.UninstallFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmapiv2.HelmRelease{
				Status: helmapiv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{},
				},
			},
			expected: ReleaseStatusProgressing,
		},
		{
			name: "ReleaseStatusProgressing",
			release: &helmapiv2.HelmRelease{
				Status: helmapiv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmapiv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			expected: ReleaseStatusProgressing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineReleaseStatus(tt.release); got != tt.expected {
				t.Errorf("DetermineReleaseStatus() = %s, want %s", got, tt.expected)
			}
		})
	}
}
