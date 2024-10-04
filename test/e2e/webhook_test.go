package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	blueprintv1alpha1 "github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
)

func TestValidationWebhook(t *testing.T) {
	f := features.New("Validation Webhook").
		Assess("RejectsInValidBlueprint", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			invalids := []blueprintv1alpha1.Blueprint{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-blueprint-1",
						Namespace: consts.NamespaceBlueprintSystem,
					},
					Spec: blueprintv1alpha1.BlueprintSpec{
						Components: blueprintv1alpha1.Component{
							Addons: []blueprintv1alpha1.AddonSpec{
								{
									Name: "addon1",
									Kind: "manifest",
									Chart: &blueprintv1alpha1.ChartInfo{
										Name: "some-chart",
										Repo: "some-repo",
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-blueprint-2",
						Namespace: consts.NamespaceBlueprintSystem,
					},
					Spec: blueprintv1alpha1.BlueprintSpec{
						Components: blueprintv1alpha1.Component{
							Addons: []blueprintv1alpha1.AddonSpec{
								{
									Name: "addon1",
									Kind: "chart",
									Manifest: &blueprintv1alpha1.ManifestInfo{
										URL: "https://some-url",
									},
								},
							},
						},
					},
				},
			}

			for _, invalid := range invalids {
				err := c.Client().Resources().Create(ctx, &invalid)
				assert.Errorf(t, err, "expected error while creating invalid blueprint")
				assert.ErrorContains(t, err, "admission webhook \"vblueprint.kb.io\" denied the request")
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, f)
}
