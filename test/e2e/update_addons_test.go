package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

// TestAddonUpdate tests the update of Helm and Manifest addons in the cluster
// It creates a blueprint with two addons, then updates the blueprint to include
// two new addons and updates the existing addons.
// It checks that the existing addons are updated to the new version
// and the new addons are installed
func TestAddonUpdate(t *testing.T) {
	dir := filepath.Join(curDir, "manifests")

	a1 := metav1.ObjectMeta{Name: "test-addon-1", Namespace: BoundlessNamespace}
	a2 := metav1.ObjectMeta{Name: "test-addon-2", Namespace: BoundlessNamespace}
	a3 := metav1.ObjectMeta{Name: "test-addon-3", Namespace: BoundlessNamespace}
	a4 := metav1.ObjectMeta{Name: "test-addon-4", Namespace: BoundlessNamespace}

	a1dep := metav1.ObjectMeta{Name: "nginx", Namespace: "test-ns-1"}
	a2dep := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}
	a3dep := metav1.ObjectMeta{Name: "crossplane", Namespace: "default"}
	a4dep := metav1.ObjectMeta{Name: "keycloak", Namespace: "default"}

	helmAddonUpdatedVersion := "15.9.1"
	manifestAddonUpdatedVersion := "v0.13.12"

	testenv.Test(t,
		features.New("Update Addons").
			WithSetup("CreatePrerequisiteBlueprint", funcs.AllOf(
				// create the blueprint with two addons, that will be updated later
				funcs.ApplyResources(FieldManager, dir, "happypath/create.yaml"),
				funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/create.yaml"),

				// wait for the addons to be installed
				funcs.AddonHaveStatusWithin(2*time.Minute, makeAddon(a1), v1alpha1.TypeComponentAvailable),
				funcs.AddonHaveStatusWithin(2*time.Minute, makeAddon(a2), v1alpha1.TypeComponentAvailable),
			)).
			WithSetup("UpdateBlueprint", funcs.AllOf(
				// update the blueprint to include two new addons and update the existing ones
				funcs.ApplyResources(FieldManager, dir, "happypath/update.yaml"),
				funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/update.yaml"),
			)).
			Assess("ExistingAddonsStillExists", funcs.AllOf(
				funcs.AddonResourcesCreatedWithin(DefaultWaitTimeout, makeAddon(a1)),
				funcs.AddonResourcesCreatedWithin(DefaultWaitTimeout, makeAddon(a2)),
			)).
			Assess("ExistingAddonsAreSuccessfullyInstalled", funcs.AllOf(
				funcs.AddonHaveStatusWithin(DefaultWaitTimeout, makeAddon(a1), v1alpha1.TypeComponentAvailable),
				funcs.AddonHaveStatusWithin(DefaultWaitTimeout, makeAddon(a2), v1alpha1.TypeComponentAvailable),
			)).
			Assess("ExistingHelmAddonIsSuccessfullyUpdated", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
				dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a1dep.Name, Namespace: a1dep.Namespace}}
				labelMatcherFunc := func(object k8s.Object) bool {
					o := object.(*appsv1.Deployment)
					actual := o.Labels["helm.sh/chart"]
					expected := fmt.Sprintf("nginx-%s", helmAddonUpdatedVersion)
					t.Logf("actual: %s, expected: %s", actual, expected)
					return actual == expected
				}

				funcs.AllOf(
					funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a1dep.Namespace, a1dep.Name),
					funcs.ResourceMatchWithin(DefaultWaitTimeout, dep, labelMatcherFunc),
				)
				return ctx
			}).
			Assess("ExistingManifestAddonIsSuccessfullyUpdated", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
				dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a2dep.Name, Namespace: a2dep.Namespace}}
				imageMatcherFunc := func(object k8s.Object) bool {
					o := object.(*appsv1.Deployment)
					imageName := o.Spec.Template.Spec.Containers[0].Image
					return strings.Contains(imageName, manifestAddonUpdatedVersion)
				}
				funcs.AllOf(
					funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a2dep.Namespace, a2dep.Name),
					funcs.ResourceMatchWithin(DefaultWaitTimeout, dep, imageMatcherFunc),
				)
				return ctx
			}).
			Assess("TwoNewAddonsAreCreated", funcs.AllOf(
				funcs.AddonResourcesCreatedWithin(DefaultWaitTimeout, makeAddon(a3)),
				funcs.AddonResourcesCreatedWithin(DefaultWaitTimeout, makeAddon(a4)),
			)).
			Assess("TwoNewAddonsAreSuccessfullyInstalled", funcs.AllOf(
				funcs.AddonHaveStatusWithin(2*time.Minute, makeAddon(a3), v1alpha1.TypeComponentAvailable),
				funcs.AddonHaveStatusWithin(2*time.Minute, makeAddon(a4), v1alpha1.TypeComponentAvailable),
			)).
			Assess("TwoNewAddonObjectsSuccessfullyCreated", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a3dep.Namespace, a3dep.Name),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, a4dep.Namespace, a4dep.Name),
			)).
			WithTeardown("DeleteBlueprint", funcs.AllOf(
				funcs.DeleteResource(makeAddon(a1)),
				funcs.DeleteResource(makeAddon(a2)),
				funcs.DeleteResource(makeAddon(a3)),
				funcs.DeleteResource(makeAddon(a4)),
				funcs.ResourceDeletedWithin(2*time.Minute, makeAddon(a1)),
				funcs.ResourceDeletedWithin(2*time.Minute, makeAddon(a2)),
				funcs.ResourceDeletedWithin(2*time.Minute, makeAddon(a3)),
				funcs.ResourceDeletedWithin(2*time.Minute, makeAddon(a4)),

				funcs.DeleteResources(dir, "happypath/update.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, dir, "happypath/update.yaml"),
			)).
			Feature(),
	)
}
