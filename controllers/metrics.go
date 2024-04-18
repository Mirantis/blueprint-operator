package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// installationHistVec is a histogram vector metric to observe various installations by Blueprint Operator.
	InstallationHistVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "blueprint_installation_histogram",
		Help: "Histogram vector for Blueprint Installations.",
		// Creating more buckets for operations that takes few seconds and less buckets
		// for those that are taking a long time.
		Buckets: []float64{1, 2, 3, 4, 5, 7, 10, 12, 15, 18, 20, 25, 30, 60, 120, 180, 300},
	},
		// Possible operations - "install", "uninstall"
		// Possible status - "pass", "fail"
		[]string{"name", "operation", "status"})
	// addOnHistVec is a histogram vector metric to observe various add ons installed by Blueprint Operator.
	AddOnHistVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "blueprint_add_on_histogram",
		Help: "Histogram vector for Blueprint Add Ons.",
		// Creating more buckets for operations that takes few seconds and less buckets
		// for those that are taking a long time.
		Buckets: []float64{1, 2, 3, 4, 5, 7, 10, 12, 15, 18, 20, 25, 30, 60, 120, 180, 300},
	},
		// Possible status - "pass", "fail"
		[]string{"name", "status"})
	// manifestHistVec is a histogram vector metric to observe various manifests reconciled by Blueprint Operator.
	ManifestHistVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "blueprint_manifest_histogram",
		Help: "Histogram vector for Blueprint Manifests.",
		// Creating more buckets for operations that takes few seconds and less buckets
		// for those that are taking a long time.
		Buckets: []float64{1, 2, 3, 4, 5, 7, 10, 12, 15, 18, 20, 25, 30, 60, 120, 180, 300},
	},
		// Possible status - "pass", "fail"
		[]string{"name", "status"})
)

const (
	// metricsOperationInstall represents an install operation by Blueprint Operator.
	metricsOperationInstall = "install"
	// metricsOperationInstall represents an uninstall operation by Blueprint Operator.
	metricsOperationUninstall = "uninstall"
	// metricStatusSuccess represents a reconcile operation by Blueprint Operator that succeeds.
	metricStatusSuccess = "success"
	// metricStatusFailure represents a reconcile operation by Blueprint Operator that fails.
	metricStatusFailure = "failure"
)

// getMetricStatus returns the status of the metric based on the error.
func getMetricStatus(err error) string {
	if err != nil {
		return metricStatusFailure
	}
	return metricStatusSuccess
}
