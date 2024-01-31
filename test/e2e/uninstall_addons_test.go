package e2e

import (
	"path/filepath"
	"testing"
	"time"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

func TestUninstallAddons(t *testing.T) {
	dir := filepath.Join(curDir, "manifests")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: BoundlessNamespace}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: BoundlessNamespace}
	a3 := metav1.ObjectMeta{Name: "test-addon-3", Namespace: BoundlessNamespace}
	a4 := metav1.ObjectMeta{Name: "test-addon-4", Namespace: BoundlessNamespace}

	a1dep := metav1.ObjectMeta{Name: "nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}
	a3dep := metav1.ObjectMeta{Name: "crossplane", Namespace: "default"}
	a4dep := metav1.ObjectMeta{Name: "keycloak", Namespace: "default"}

	f := features.New("Uninstall Addons").
		WithSetup("CreatePrerequisiteAddons", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),

			// ensure all addons are installed and available before we start deleting them
			funcs.AddonHaveStatusWithin(2*time.Minute, makeAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, makeAddon(a2), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(2*time.Minute, makeAddon(a3), v1alpha1.TypeComponentAvailable),
			funcs.AddonHaveStatusWithin(5*time.Minute, makeAddon(a4), v1alpha1.TypeComponentAvailable),
		)).
		WithSetup("DeleteAddonsWithBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/delete.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/delete.yaml"),
		)).
		Assess("AllRemovedAddonsHaveBeenDeleted", funcs.AllOf(
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, makeAddon(a2)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, makeAddon(a3)),
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, makeAddon(a4)),
		)).
		Assess("Addon2ObjectsHasBeenDeleted", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a2dep.Name, Namespace: a2dep.Namespace}}),
		)).
		Assess("Addon3ObjectsHasBeenDeleted", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a3dep.Name, Namespace: a3dep.Namespace}}),
		)).
		Assess("Addon4ObjectsHasBeenDeleted", funcs.AllOf(
			// @TODO: check for more/all objects
			funcs.ResourceDeletedWithin(DefaultWaitTimeout, &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a4dep.Name, Namespace: a4dep.Namespace}}),
		)).
		Assess("Addon1StillAvailable", funcs.AllOf(
			funcs.AddonHaveStatusWithin(DefaultWaitTimeout, makeAddon(a1), v1alpha1.TypeComponentAvailable),
			funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			funcs.DeleteResource(makeAddon(a1)),
			funcs.ResourceDeletedWithin(2*time.Minute, makeAddon(a1)),

			funcs.DeleteResources(dir, "happypath/delete.yaml"),
			funcs.ResourcesDeletedWithin(2*time.Minute, dir, "happypath/delete.yaml"),
		)).
		Feature()

	testenv.Test(t, f)
}
