package e2e

import (
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

func TestOperatorInstall(t *testing.T) {
	testenv.Test(t,
		features.New("Blueprint Operator Installation").
			Assess("BlueprintOperatorDeploymentIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBlueprintSystem, consts.BlueprintOperatorName),
			)).
			Assess("HelmControllerIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceFluxSystem, "helm-controller"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceFluxSystem, "source-controller"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceFluxSystem, "kustomize-controller"),
			)).
			Assess("CertManagerIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBlueprintSystem, "cert-manager"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBlueprintSystem, "cert-manager-webhook"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBlueprintSystem, "cert-manager-cainjector"),
			)).
			Assess("WebhookIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBlueprintSystem, "blueprint-operator-webhook"),
			)).
			Feature(),
	)
}
