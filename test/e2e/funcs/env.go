package funcs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
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
		_ = certmanager.AddToScheme(cfg.Client().Resources().GetScheme())
		return ctx, nil
	}
}

// InstallBoundlessOperator installs the boundless operator
func InstallBoundlessOperator(img string) env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		wd, err := os.Getwd()
		if err != nil {
			return ctx, fmt.Errorf("failed to get working directory: %v", err)
		}
		wd = strings.Replace(wd, "/test/e2e", "", -1)
		dir := fmt.Sprintf("%s/deploy/static", wd)

		updateImageFunc := decoder.MutateOption(func(o k8s.Object) error {
			if o.GetObjectKind().GroupVersionKind().Kind == "Deployment" {
				deployment, ok := o.(*appsv1.Deployment)
				if !ok {
					return fmt.Errorf("unexpected type %T not Deployment", o)
				}
				for i, container := range deployment.Spec.Template.Spec.Containers {
					if container.Name == consts.BoundlessContainerName {
						deployment.Spec.Template.Spec.Containers[i].Image = img
					}
				}
			}
			return nil
		})

		if err = decoder.ApplyWithManifestDir(ctx, c.Client().Resources(), dir, "blueprint-operator.yaml", []resources.CreateOption{}, updateImageFunc); err != nil {
			return ctx, fmt.Errorf("failed to install boundless operator: %v", err)
		}

		// Wait for the boundless operator to be ready
		if err = waitForDeploymentReady(c, consts.NamespaceBoundlessSystem, consts.BoundlessOperatorName, 5*time.Minute); err != nil {
			return ctx, fmt.Errorf("failed to wait for boundless operator to be ready: %v", err)
		}

		// Wait for the boundless operator webhook to be ready
		if err = waitForDeploymentReady(c, consts.NamespaceBoundlessSystem, consts.BoundlessOperatorWebhookName, 5*time.Minute); err != nil {
			return ctx, fmt.Errorf("failed to wait for boundless operator webhook to be ready: %v", err)
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
	list := &appsv1.DeploymentList{
		Items: []appsv1.Deployment{
			{
				ObjectMeta: v1.ObjectMeta{Name: name, Namespace: namespace},
			},
		},
	}
	if err := wait.For(conditions.New(c.Client().Resources()).ResourcesFound(list), wait.WithTimeout(timeout), wait.WithInterval(DefaultPollInterval)); err != nil {
		return fmt.Errorf("failed to wait for deployment to be created within %s: %v", timeout, err)
	}

	if err := wait.For(conditions.New(c.Client().Resources()).DeploymentAvailable(name, namespace), wait.WithTimeout(timeout), wait.WithInterval(DefaultPollInterval)); err != nil {
		return fmt.Errorf("failed to wait for deployment to be ready within %s: %v", timeout, err)
	}

	return nil
}
