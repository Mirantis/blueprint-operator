package e2e

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

// TestManifestKustomize tests the manifest addon with kustomize
func TestManifestKustomize(t *testing.T) {
	dir := filepath.Join(curDir, "manifests", "addons")

	a1 := metav1.ObjectMeta{Name: "metallb-kustomize", Namespace: consts.NamespaceBlueprintSystem}

	// Expected values after applying the kustomize patches
	expectedMetallbControllerImageVersion := "v0.13.12"
	expectedFailureThreshold := int32(2)

	deploy := metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}
	daemon := metav1.ObjectMeta{Name: "speaker", Namespace: "metallb-system"}

	f := features.New("Install Addons with Kustomize patches").
		WithSetup("CreateBlueprint", funcs.AllOf(
			funcs.ApplyResources(FieldManager, dir, "happypath/kustomize.yaml"),
			funcs.ResourcesCreatedWithin(DefaultWaitTimeout, dir, "happypath/kustomize.yaml"),
		)).
		Assess("AddonIsCreated", funcs.AllOf(
			funcs.ComponentResourcesCreatedWithin(DefaultWaitTimeout, newAddon(a1)),
		)).
		Assess("AddonsIsInstalled", funcs.AllOf(
			funcs.AddonHaveStatusWithin(2*time.Minute, newAddon(a1), v1alpha1.TypeComponentAvailable),
		)).
		Assess("KustomizeImageAppliedCorrectly", funcs.AllOf(
			funcs.ResourceMatchWithin(DefaultWaitTimeout, &appsv1.Deployment{ObjectMeta: deploy}, func(object k8s.Object) bool {
				o := object.(*appsv1.Deployment)
				imageName := o.Spec.Template.Spec.Containers[0].Image
				return strings.Contains(imageName, expectedMetallbControllerImageVersion)
			}),
			funcs.ResourceMatchWithin(DefaultWaitTimeout, &appsv1.DaemonSet{ObjectMeta: daemon}, func(object k8s.Object) bool {
				o := object.(*appsv1.DaemonSet)
				imageName := o.Spec.Template.Spec.Containers[0].Image
				return strings.Contains(imageName, expectedMetallbControllerImageVersion)
			}),
		)).
		Assess("KustomizePatchAppliedCorrectly", funcs.AllOf(
			funcs.ResourceMatchWithin(DefaultWaitTimeout, &appsv1.Deployment{ObjectMeta: deploy}, func(object k8s.Object) bool {
				o := object.(*appsv1.Deployment)
				return o.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold == expectedFailureThreshold
			}),
		)).
		WithTeardown("Cleanup", funcs.AllOf(
			ApplyCleanupBlueprint(),
			funcs.DeleteResources(dir, "happypath/kustomize.yaml"),
			funcs.ResourceDeletedWithin(2*time.Minute, newAddon(a1)),
		)).
		Feature()

	testenv.Test(t, f)
}
