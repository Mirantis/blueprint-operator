package test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	boundlessOperatorDeploymentName      = "boundless-operator-controller-manager"
	boundlessOperatorDeploymentNamespace = "boundless-system"
)

var _ = Describe("Boundless Operator e2e test", func() {
	// Simple e2e test that installs boundless operator and verifies the installation.
	It("verifying installation of the boundless operator", func() {
		framework.ExpectNoError(InstallBoundlessOperator(), "failed to install boundless operator")
	})

})

// InstallBoundlessOperator installs the boundless operator from deploy/static/boundless-operator.yaml.
func InstallBoundlessOperator() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	wd = strings.Replace(wd, "/test", "", -1)
	url := fmt.Sprintf("%s/deploy/static/boundless-operator.yaml", wd)
	By(fmt.Sprintf("Installing boundless operator from the manifest located at: %s", url))
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}

	cmd = exec.Command("kubectl", "wait", fmt.Sprintf("deployment.apps/%s", boundlessOperatorDeploymentName),
		"--for", "condition=Available",
		"--namespace", fmt.Sprintf("%s", boundlessOperatorDeploymentNamespace),
		"--timeout", "5m",
	)
	By(fmt.Sprintf("Waiting for deployment to be ready by running command: %s", cmd.Args))
	output, err := cmd.CombinedOutput()
	if err != nil {
		framework.Logf("Failed to execute wait command: %s", string(output))
		return err
	}
	framework.Logf("%s", string(output))
	return err
}
