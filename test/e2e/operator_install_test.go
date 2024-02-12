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
			Assess("HelmControllerDeploymentIsSuccessfullyInstalled", funcs.AllOf(
				funcs.DeploymentBecomesAvailableWithin(DefaultWaitTimeout, consts.NamespaceBoundlessSystem, "helm-controller"),
			)).
			Feature(),
	)
}
