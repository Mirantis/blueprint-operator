package bopinstall

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	boundlessOperatorDeploymentName      = "boundless-operator-controller-manager"
	boundlessOperatorDeploymentNamespace = "boundless-system"
)

func TestBOPInstall(t *testing.T) {
	f := features.New("Boundless operator").
		Assess("Install BOP", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatal(err)
			}
			wd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			wd = strings.Replace(wd, "/test/e2e/bopinstall", "", -1)
			dir := fmt.Sprintf("%s/deploy/static", wd)

			t.Logf("Installing boundless operator from the manifest located at:%s", dir)

			err = decoder.ApplyWithManifestDir(ctx, r, dir, "*", []resources.CreateOption{})
			if err != nil {
				t.Fatal(err)
			}

			return ctx
		}).
		Assess("Check BOP deployment available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: boundlessOperatorDeploymentName, Namespace: boundlessOperatorDeploymentNamespace},
			}
			t.Logf("Waiting for deployment : %s to be ready.", boundlessOperatorDeploymentName)
			// wait for the deployment to finish becoming available
			err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(time.Minute*1))
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Deployment %s is available. ReadyReplicas: %d", boundlessOperatorDeploymentName, dep.Status.ReadyReplicas)
			return ctx

		}).Feature()

	_ = testenv.Test(t, f)
}
