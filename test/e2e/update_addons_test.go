package e2e

import (
	"fmt"
	"github.com/mirantiscontainers/blueprint-operator/client/api/v1alpha1"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

// TestUpdateAddons tests the update of Helm and Manifest addons in the cluster
// 1. Creates a blueprint with two addons,
// 2. Updates the blueprint to include two new addons and updates the existing addons.
// 3. Checks that the existing addons are updated to the new version
// 4. Checks that the new addons are installed
func TestUpdateAddons(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "addons")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: consts.NamespaceBlueprintSystem}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: consts.NamespaceBlueprintSystem}
	a3 := metav1.ObjectMeta{Name: "test-addon-3", Namespace: consts.NamespaceBlueprintSystem}

	a1dep := metav1.ObjectMeta{Name: "test-addon-1-nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}
	a3dep := metav1.ObjectMeta{Name: "crossplane", Namespace: "default"}

	helmAddonUpdatedVersion := "16.0.7"
	manifestAddonUpdatedVersion := "v0.13.12"

	f := features.New("Update Addons").
		WithSetup("CreatePrerequisiteBlueprint", funcs.AllOf(
			// create the blueprint with two addons that will be updated later
			funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),

			// wait for the addons to be installed
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a2), v1alpha1.TypeComponentAvailable),
		)).
		WithSetup("UpdateBlueprint", funcs.AllOf(
			// update the blueprint to include two new addons and update the existing ones
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),
		)).
		Assess("ExistingAddonsStillExists", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a1)),
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a2)),
		)).
		Assess("ExistingAddonsAreSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(DefaultWaitTimeout, newAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(DefaultWaitTimeout, newAddon(a2), v1alpha1.TypeComponentAvailable),
		)).
		Assess("ExistingHelmAddonIsSuccessfullyUpdated", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
			funcs.ResourceMatchWithin(DefaultWaitTimeout, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a1dep.Name, Namespace: a1dep.Namespace}}, func(object k8s.Object) bool {
				// check that the helm label of the deployment has been updated
				o := object.(*appsv1.Deployment)
				actual := o.Labels["helm.sh/chart"]
				expected := fmt.Sprintf("nginx-%s", helmAddonUpdatedVersion)
				t.Logf("actual: %s, expected: %s", actual, expected)
				return actual == expected
			}),
		)).
		Assess("ExistingManifestAddonIsSuccessfullyUpdated", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a2dep.Namespace, a2dep.Name),
			funcs.ResourceMatchWithin(DefaultWaitTimeout, &appsv1.Deployment{ObjectMeta: a2dep}, func(object k8s.Object) bool {
				o := object.(*appsv1.Deployment)
				imageName := o.Spec.Template.Spec.Containers[0].Image
				return strings.Contains(imageName, manifestAddonUpdatedVersion)
			}),
		)).
		Assess("NewAddonsAreCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a3)),
		)).
		Assess("NewAddonsAreSuccessfullyInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a3), v1alpha1.TypeComponentAvailable),
		)).
		Assess("NewAddonObjectsSuccessfullyCreated", funcs.AllOf(
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a3dep.Namespace, a3dep.Name),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a2)),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a3)),
		)).
		Feature()

	testenv.Test(t, f)
}
