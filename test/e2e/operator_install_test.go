package e2e

import (
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

func TestOperatorInstall(t *testing.T) {
	testenv.Test(t,
		features.New("Boundless Operator Installation").
			Assess("BoundlessOperatorDeploymentIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, BoundlessNamespace, BoundlessOperatorName),
			)).
			Assess("HelmControllerDeploymentIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, BoundlessNamespace, "helm-controller"),
			)).
			Feature(),
	)
}
