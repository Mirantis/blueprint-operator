package e2e

import (
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

func TestOperatorInstall(t *testing.T) {
	testenv.Test(t,
		features.New("Boundless Operator Installation").
			Assess("BoundlessOperatorDeploymentIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, consts.BoundlessOperatorName),
			)).
			Assess("HelmControllerIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceFluxSystem, "helm-controller"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceFluxSystem, "source-controller"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceFluxSystem, "kustomize-controller"),
			)).
			Assess("CertManagerIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager-webhook"),
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "cert-manager-cainjector"),
			)).
			Assess("WebhookIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "blueprint-operator-webhook"),
			)).
			Feature(),
	)
}
