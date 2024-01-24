package happypath

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

var (
	testenv env.Environment
)

var (
	addonName1 = "test-addon-1"
	addonName2 = "test-addon-2"
	addonName3 = "test-addon-3"
	addonName4 = "test-addon-4"

	helmAddonUpdatedVersion     = "15.9.1"
	manifestAddonUpdatedVersion = "v0.13.12"
)
var curDir, _ = os.Getwd()

func TestBlueprintHappyPath(t *testing.T) {
	f1 := AddonInstallFeature()
	f2 := AddonUpdateFeature()
	f3 := AddonUninstallFeature()

	// test feature
	testenv.Test(t, f1, f2, f3)
}

func AddonInstallFeature() features.Feature {
	f := features.New("Install Addons").
		Setup(setupClientScheme()).
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			path := filepath.Join(curDir, "testdata", "create")
			t.Logf("applying blueprint from directory: %s", path)
			err := createManifest(ctx, cfg, path)
			assert.NoError(t, err, "failed to apply blueprint from directory: %s", path)
			return ctx
		}).
		Assess("creates addons", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// wait for addons to be created
			minExpectedCount := 2
			err := wait.For(conditions.New(client.Resources(namespace)).ResourceListN(&v1alpha1.AddonList{}, minExpectedCount), wait.WithTimeout(waitTimeout))
			assert.NoError(t, err, "failed to wait for addons")

			// check correct addons are created
			expectedAddons := []string{addonName1, addonName2}
			addons := v1alpha1.AddonList{}
			err = client.Resources(namespace).List(ctx, &addons)
			assert.NoError(t, err, "failed to list addons")

			for _, addon := range addons.Items {
				assert.Contains(t, expectedAddons, addon.Name, "expected addon not found: %s", addon.Name)
			}
			return ctx

		}).
		Assess("helm addon is marked as successfully installed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// wait for helm addon to be successfully installed
			addon := v1alpha1.Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      addonName1,
					Namespace: namespace,
				},
			}

			err := waitForAddonStatus(client, &addon, v1alpha1.TypeComponentAvailable, waitTimeout)
			assert.NoError(t, err, "helm addon status is not as expected, expected: %s, got: %s", v1alpha1.TypeComponentAvailable, addon.Status.Type)

			return ctx
		}).
		Assess("manifest addon is marked as successfully installed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// wait for manifest addon to be successfully installed
			addon := v1alpha1.Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      addonName2,
					Namespace: namespace,
				},
			}

			err := waitForAddonStatus(client, &addon, v1alpha1.TypeComponentAvailable, waitTimeout)
			assert.NoError(t, err, "manifest addon status is not as expected, expected: %s, got: %s", v1alpha1.TypeComponentAvailable, addon.Status.Type)

			return ctx
		}).
		Assess("helm addon objects are successfully created", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// check helm addon objects are created
			// @todo: check for more objects
			deps := &appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "test-ns-1"}},
				},
			}
			err := wait.For(conditions.New(client.Resources()).ResourcesFound(deps), wait.WithTimeout(waitTimeout))
			assert.NoError(t, err, "failed to find expected objects")
			return ctx
		}).
		Assess("manifest addon objects are successfully created", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			var objs []k8s.Object
			objs = append(objs, &v1.Service{})
			objs = append(objs, &v1.Pod{})

			// check manifest addon objects are created
			// @todo: check for more objects here
			deps := &appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}},
				},
			}
			err := wait.For(conditions.New(client.Resources()).ResourcesFound(deps), wait.WithTimeout(waitTimeout))
			assert.NoError(t, err, "failed to find expected objects")
			return ctx
		}).
		Feature()

	return f
}

func AddonUpdateFeature() features.Feature {
	f := features.New("Update Addons").
		Setup(setupClientScheme()).
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			path := filepath.Join(curDir, "testdata", "update")
			t.Logf("applying blueprint from directory: %s", path)
			err := updateManifest(ctx, cfg, path)
			assert.NoError(t, err, "failed to apply blueprint from directory: %s", path)
			return ctx
		}).
		Assess("creates new addons from blueprint", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// wait for addons to be created
			minExpectedCount := 4
			err := wait.For(conditions.New(client.Resources(namespace)).ResourceListN(&v1alpha1.AddonList{}, minExpectedCount), wait.WithTimeout(waitTimeout))
			assert.NoError(t, err, "failed to wait for addons")

			// check correct addons are created
			expectedAddons := []string{addonName1, addonName2, addonName3, addonName4}
			addons := v1alpha1.AddonList{}
			err = client.Resources(namespace).List(ctx, &addons)
			assert.NoError(t, err, "failed to list addons")

			for _, addon := range addons.Items {
				assert.Contains(t, expectedAddons, addon.Name, "expected addon not found: %s", addon.Name)
			}
			return ctx

		}).
		Assess("all addons are marked as successfully installed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// wait for helm addon to be successfully installed
			addons := []v1alpha1.Addon{
				{ObjectMeta: metav1.ObjectMeta{Name: addonName1, Namespace: namespace}},
				{ObjectMeta: metav1.ObjectMeta{Name: addonName2, Namespace: namespace}},
				{ObjectMeta: metav1.ObjectMeta{Name: addonName3, Namespace: namespace}},
				{ObjectMeta: metav1.ObjectMeta{Name: addonName4, Namespace: namespace}},
			}

			for _, addon := range addons {
				err := waitForAddonStatus(client, &addon, v1alpha1.TypeComponentAvailable, waitTimeout)
				assert.NoError(t, err, "helm addon status is not as expected for %s/%s, expected: %s, got: %s", addon.Spec.Kind, addon.Spec.Name, v1alpha1.TypeComponentAvailable, addon.Status.Type)
			}

			return ctx
		}).
		Assess("existing helm addon has been successfully updated", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			dep := appsv1.Deployment{}
			err := client.Resources().Get(ctx, "nginx", "test-ns-1", &dep)
			assert.NoError(t, err, "failed to get deployment %w", err)

			actual := dep.Labels["helm.sh/chart"]
			expected := fmt.Sprintf("nginx-%s", helmAddonUpdatedVersion)
			assert.Equal(t, expected, actual, "helm addon has not been updated")
			return ctx
		}).
		Assess("existing manifest addon has been successfully updated", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			dep := appsv1.Deployment{}
			err := client.Resources().Get(ctx, "controller", "metallb-system", &dep)
			assert.NoError(t, err, "failed to get deployment: %w", err)

			imageName := dep.Spec.Template.Spec.Containers[0].Image
			assert.Contains(t, imageName, manifestAddonUpdatedVersion, "manifest addon has not been updated")
			return ctx
		}).
		Feature()
	return f
}

func AddonUninstallFeature() features.Feature {
	f := features.New("Uninstall Addons").Setup(setupClientScheme()).
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			path := filepath.Join(curDir, "testdata", "delete")
			t.Logf("applying blueprint from directory: %s", path)
			err := updateManifest(ctx, cfg, path)
			assert.NoError(t, err, "failed to apply blueprint from directory: %s", path)
			return ctx
		}).
		Assess("all addons have been deleted", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// wait for helm addon to be successfully delete
			list := v1alpha1.AddonList{
				Items: []v1alpha1.Addon{
					{ObjectMeta: metav1.ObjectMeta{Name: addonName1, Namespace: namespace}},
					{ObjectMeta: metav1.ObjectMeta{Name: addonName2, Namespace: namespace}},
					{ObjectMeta: metav1.ObjectMeta{Name: addonName3, Namespace: namespace}},

					// @todo: this is causing issues with the test, need to investigate
					// {ObjectMeta: metav1.ObjectMeta{Name: addonName4, Namespace: namespace}},
				},
			}

			err := wait.For(conditions.New(client.Resources(namespace)).ResourcesDeleted(&list), wait.WithTimeout(waitTimeout))
			assert.NoError(t, err, "all addons have not been deleted: %w", err)

			return ctx
		}).
		Assess("helm addon objects have been deleted", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// @todo: check for more objects
			list := appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "test-ns-1"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "test-ns-3"}},
				},
			}

			err := wait.For(conditions.New(client.Resources()).ResourcesDeleted(&list), wait.WithTimeout(waitTimeout))
			assert.NoError(t, err, "helm addon objects have not been deleted: %w", err)
			return ctx
		}).
		Assess("manifest addon objects have been deleted", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// @todo: check for more objects
			list := appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "metallb-system"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "ingress-nginx-controller", Namespace: "ingress-nginx"}},
				},
			}

			err := wait.For(conditions.New(client.Resources()).ResourcesDeleted(&list), wait.WithTimeout(waitTimeout))
			assert.NoError(t, err, "helm addon objects have not been deleted: %w", err)
			return ctx
		}).
		Feature()
	return f
}
