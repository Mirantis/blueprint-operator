package happypath

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

const (
	namespace    = "boundless-system"
	operatorName = "boundless-operator-controller-manager"
	waitTimeout  = time.Minute * 1
)

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("test-cluster", 16)

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),

		// install boundless operator
		InstallBoundlessOperator(),
	)

	testenv.BeforeEachFeature()

	testenv.Finish(
		envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}

// InstallBoundlessOperator installs the boundless operator
func InstallBoundlessOperator() env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		r, err := resources.New(config.Client().RESTConfig())
		if err != nil {
			return ctx, err
		}
		wd, err := os.Getwd()
		if err != nil {
			return ctx, err
		}
		wd = strings.Replace(wd, "/test/e2e/happypath", "", -1)
		dir := fmt.Sprintf("%s/deploy/static", wd)

		if err = decoder.ApplyWithManifestDir(ctx, r, dir, "*", []resources.CreateOption{}); err != nil {
			return ctx, fmt.Errorf("failed to install boundless operator: %v", err)
		}

		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      operatorName,
				Namespace: namespace,
			},
		}

		// wait for operator to be ready
		if err = waitForDeploymentReady(config.Client(), &dep, waitTimeout); err != nil {
			return ctx, fmt.Errorf("failed to wait for boundless operator to be ready: %v", err)
		}

		return ctx, nil
	}
}

func setupClientScheme() func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		// add v1alpha1 to scheme so we can access addons
		client := cfg.Client()
		err := v1alpha1.AddToScheme(client.Resources().GetScheme())
		assert.NoError(t, err, "failed to add v1alpha1 to scheme")
		return ctx
	}
}

func waitForAddonStatus(client klient.Client, addon *v1alpha1.Addon, expectedType v1alpha1.StatusType, timeout time.Duration) error {
	return wait.For(conditions.New(client.Resources()).ResourceMatch(addon, func(object k8s.Object) bool {
		a := object.(*v1alpha1.Addon)
		return a.Status.Type == expectedType
	}), wait.WithTimeout(timeout))
}

func waitForDeploymentReady(client klient.Client, dep *appsv1.Deployment, timeout time.Duration) error {
	return wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(dep, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(timeout))
}

func createManifest(ctx context.Context, cfg *envconf.Config, dir string) error {
	r, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		return err
	}
	if err = decoder.ApplyWithManifestDir(ctx, r, dir, "*", []resources.CreateOption{}); err != nil {
		return err
	}
	return nil
}

func updateManifest(ctx context.Context, cfg *envconf.Config, dir string) error {
	client := cfg.Client()
	blueprint := v1alpha1.Blueprint{}
	err := client.Resources(namespace).Get(ctx, "boundless-cluster", namespace, &blueprint)
	if err != nil {
		return fmt.Errorf("failed to get existing blueprint: %w", err)
	}

	// update blueprint to set resource version
	setResourceVersionFunc := func(obj k8s.Object) error {
		obj.SetResourceVersion(blueprint.ResourceVersion)
		return nil
	}

	decoderOpts := decoder.MutateOption(setResourceVersionFunc)

	r, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		return err
	}
	if err = UpdateWithManifestDir(ctx, r, dir, "*", []resources.UpdateOption{}, decoderOpts); err != nil {
		return err
	}
	return nil
}

// UpdateWithManifestDir resolves all the files in the Directory dirPath against the globbing pattern and creates a kubernetes
// resource for each of the resources found under the manifest directory.
func UpdateWithManifestDir(ctx context.Context, r *resources.Resources, dirPath, pattern string, updateOptions []resources.UpdateOption, options ...decoder.DecodeOption) error {
	err := decoder.DecodeEachFile(ctx, os.DirFS(dirPath), pattern, decoder.UpdateHandler(r, updateOptions...), options...)
	return err
}
