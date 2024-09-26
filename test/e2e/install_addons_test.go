package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

var curDir, _ = os.Getwd()

// TestInstallAddons tests the installation of two addons, one Helm addon and one Manifest addon.
// It checks if the addons are created, installed and their objects are created
func TestInstallAddons(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "addons")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: consts.NamespaceBoundlessSystem}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: consts.NamespaceBoundlessSystem}

	a1dep := metav1.ObjectMeta{Name: "test-addon-1-nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}

	f := features.New("Install Addons").
		WithSetup("CreateBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),
		)).
		Assess("TwoAddonsAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a1)),
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a2)),
		)).
		Assess("HelmAddonsIsSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
		)).
		Assess("ManifestAddonIsSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a2), v1alpha1.TypeComponentAvailable),
		)).
		Assess("HelmAddonObjectsAreSuccessfullyCreated", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
		)).
		Assess("ManifestAddonObjectsAreSuccessfullyCreated", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a2dep.Namespace, a2dep.Name),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a2)),
		)).
		Feature()

	testenv.Test(t, f)
}

// TestInstallAddons_MultipleAddonsWithSameChart tests the installation of two addons with the same chart.
func TestInstallAddons_MultipleAddonsWithSameChart(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "addons")

	a1 := metav1.ObjectMeta{Name: "same-chart-1", Namespace: consts.NamespaceBoundlessSystem}
	a2 := metav1.ObjectMeta{Name: "same-chart-2", Namespace: consts.NamespaceBoundlessSystem}

	a1dep := metav1.ObjectMeta{Name: "same-chart-1-nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "same-chart-2-nginx", Namespace: "test-ns-1"}

	f := features.New("Install Multiple Addons With Same Chart").
		WithSetup("CreateBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/multiaddon.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/multiaddon.yaml"),
		)).
		Assess("TwoHelmAddonsUsingSameAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a1)),
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a2)),
		)).
		Assess("HelmAddonsUsingSameChartAreSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a2), v1alpha1.TypeComponentAvailable),
		)).
		Assess("HelmAddonObjectsAreSuccessfullyCreated", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a2dep.Namespace, a2dep.Name),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a2)),
		)).
		Feature()

	testenv.Test(t, f)
}
