package funcs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
)

// AddBoundlessTypeToScheme adds the boundless operator's custom resources to the environment's scheme
// so that the environment's client can work with these types.
func AddBoundlessTypeToScheme() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		_ = v1alpha1.AddToScheme(cfg.Client().Resources().GetScheme())
		return ctx, nil
	}
}

// InstallBoundlessOperator installs the boundless operator
func InstallBoundlessOperator() env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		wd, err := os.Getwd()
		if err != nil {
			return ctx, fmt.Errorf("failed to get working directory: %v", err)
		}
		wd = strings.Replace(wd, "/test/e2e", "", -1)
		dir := fmt.Sprintf("%s/deploy/static", wd)

		if err = decoder.ApplyWithManifestDir(ctx, c.Client().Resources(), dir, "*", []resources.CreateOption{}); err != nil {
			return ctx, fmt.Errorf("failed to install boundless operator: %v", err)
		}

		// Wait for the boundless operator to be ready
		if err = waitForDeploymentReady(c, consts.NamespaceBoundlessSystem, consts.BoundlessOperatorName, 5*time.Minute); err != nil {
			return ctx, fmt.Errorf("failed to wait for boundless operator to be ready: %v", err)
		}
		return ctx, nil
	}
}

// SleepFor is a feature function that sleeps for a given duration after running a feature
func SleepFor(d time.Duration) env.FeatureFunc {
	return func(ctx context.Context, c *envconf.Config, t *testing.T, f features.Feature) (context.Context, error) {
		t.Logf("Sleeping for %s after running feature %s", d.String(), f.Name())
		time.Sleep(d)
		return ctx, nil
	}
}

// waitForDeploymentReady waits for the deployment to be ready.
func waitForDeploymentReady(c *envconf.Config, namespace, name string, timeout time.Duration) error {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	// wait for the deployment to be ready
	if err := wait.For(conditions.New(c.Client().Resources()).DeploymentConditionMatch(dep, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(timeout), wait.WithInterval(DefaultPollInterval)); err != nil {
		return fmt.Errorf("failed to wait for deployment to be ready: %v", err)
	}

	return nil
}
