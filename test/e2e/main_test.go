package e2e

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"

	"github.com/mirantiscontainers/blueprint-operator/test/e2e/funcs"
)

var (
	testenv Environment
)

// The caller (e.g. make e2e) must ensure these exist.
const (
	defaultOperatorImage = "ghcr.io/mirantiscontainers/blueprint-operator:latest"
)

func TestMain(m *testing.M) {
	testenv = NewEnvironmentFromFlags()
	kindClusterName := envconf.RandomName("test-cluster", 32)

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		panic(err)
	}
	testenv.SetEnvironment(env.NewWithConfig(cfg))

	operatorImage := testenv.GetOperatorImage()

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),

		// load image into kind cluster
		envfuncs.LoadDockerImageToCluster(kindClusterName, operatorImage),

		// add blueprint types to scheme
		funcs.AddBlueprintTypeToScheme(),

		// install blueprint operator
		funcs.InstallBlueprintOperator(operatorImage),
	)

	testenv.Finish(
		envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}
