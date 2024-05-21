package helm

import (
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetermineReleaseStatus(t *testing.T) {
	tests := []struct {
		name     string
		release  *helmv2.HelmRelease
		expected string
	}{
		{
			name: "ReleaseStatusSuccess",
			release: &helmv2.HelmRelease{
				Status: helmv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmv2.ReleasedCondition,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: ReleaseStatusSuccess,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmv2.HelmRelease{
				Status: helmv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmv2.InstallFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmv2.HelmRelease{
				Status: helmv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmv2.UpgradeFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmv2.HelmRelease{
				Status: helmv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmv2.RollbackFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmv2.HelmRelease{
				Status: helmv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmv2.ReleasedCondition,
							Status: metav1.ConditionFalse,
							Reason: helmv2.UninstallFailedReason,
						},
					},
				},
			},
			expected: ReleaseStatusFailed,
		},
		{
			name: "ReleaseStatusFailed",
			release: &helmv2.HelmRelease{
				Status: helmv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{},
				},
			},
			expected: ReleaseStatusProgressing,
		},
		{
			name: "ReleaseStatusProgressing",
			release: &helmv2.HelmRelease{
				Status: helmv2.HelmReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   helmv2.ReleasedCondition,
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
