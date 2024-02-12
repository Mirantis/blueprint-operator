package e2e

import (
	"os"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"

	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

var (
	testenv Environment
)

// The caller (e.g. make e2e) must ensure these exist.
const (
	defaultOperatorImage = "ghcr.io/mirantiscontainers/boundless-operator:latest"
)

func TestMain(m *testing.M) {
	testenv = NewEnvironmentFromFlags()
	kindClusterName := envconf.RandomName("test-cluster", 32)

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		panic(err)
	}
	testenv.SetEnvironment(env.NewWithConfig(cfg))

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),

		// load image into kind cluster
		envfuncs.LoadDockerImageToCluster(kindClusterName, testenv.GetOperatorImage()),

		// add boundless types to scheme
		funcs.AddBoundlessTypeToScheme(),

		// install boundless operator
		funcs.InstallBoundlessOperator(),
	)

	testenv.AfterEachFeature(
		// When we run cleanup after a test feature that applies empty Blueprint,
		// we need to wait for the kube controller to reconcile the deletion of the addons.
		// The checks that we do in the test does not guarantee that the controller has finished
		// deleting addons
		funcs.SleepFor(10 * time.Second),
	)

	testenv.Finish(
		envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}
