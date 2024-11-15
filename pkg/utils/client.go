package utils

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MustLabelSelector creates a new label selector with the given key, operator, and values
// panics if any of the values are invalid
func MustLabelSelector(key string, operator selection.Operator, values []string) client.MatchingLabelsSelector {
	req, err := labels.NewRequirement(key, operator, values)
	if err != nil {
		panic("panic creating label selector requirement: " + err.Error())
	}

	selector := labels.NewSelector().Add(*req)

	return client.MatchingLabelsSelector{Selector: selector}
}
