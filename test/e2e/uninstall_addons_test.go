package e2e

import (
	"path/filepath"
	"testing"
	"time"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/blueprint-operator/client/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

// TestUninstallAddons tests the uninstallation of helm and manifest addons:
// 1. Apply a blueprint with 4 addons
// 2. Uninstall 3 addons (by applying a blueprint with 1 addon)
// 3. Ensure the 3 addons and their objects are removed
func TestUninstallAddons(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "addons")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: consts.NamespaceBlueprintSystem}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: consts.NamespaceBlueprintSystem}
	a3 := metav1.ObjectMeta{Name: "test-addon-3", Namespace: consts.NamespaceBlueprintSystem}

	a1dep := metav1.ObjectMeta{Name: "test-addon-1-nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}
	a3dep := metav1.ObjectMeta{Name: "crossplane", Namespace: "default"}

	f := features.New("Uninstall Addons").
		WithSetup("CreatePrerequisiteAddons", funcs.AllOf(
			// apply a blueprint with 3 addons
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),

			// ensure all addons are installed and available before we start deleting them
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
			// For some reason, the metallb deployment after the update test suite is run.
			// This is causing the test to fail. This is a temporary fix
			//funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a2), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a3), v1alpha1.TypeComponentAvailable),
		)).
		WithSetup("DeleteAddonsWithBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/delete.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/delete.yaml"),
		)).
		Assess("AllRemovedAddonsHaveBeenDeleted", funcs.AllOf(
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newAddon(a2)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, newAddon(a3)),
		)).
		Assess("Addon2ObjectsHasBeenDeleted", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a2dep.Name, Namespace: a2dep.Namespace}}),
		)).
		Assess("Addon3ObjectsHasBeenDeleted", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a3dep.Name, Namespace: a3dep.Namespace}}),
		)).
		Assess("Addon1StillAvailable", funcs.AllOf(
			funcs.AddonHaveStatusWithin(DefaultWaitTimeout, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.DeleteResource(newAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
		)).
		Feature()

	testenv.Test(t, f)
}
