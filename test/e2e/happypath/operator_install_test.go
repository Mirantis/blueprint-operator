package happypath

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestBOPInstall(t *testing.T) {
	f := features.New("Boundless Operator Installation").
		Assess("boundless operator is successfully installed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: operatorName, Namespace: namespace},
			}

			err = waitForDeploymentReady(client, &dep, waitTimeout)
			assert.NoError(t, err, "failed to wait for deployment %q to be ready: %w", operatorName, err)
			return ctx

		}).Feature()

	_ = testenv.Test(t, f)
}
