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
	testenv env.Environment
)

// The caller (e.g. make e2e) must ensure these exist.
const (
	boundlessImage = "mirantiscontainers/boundless-operator:latest"
)

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("test-cluster", 16)

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),

		// load images into kind cluster
		envfuncs.LoadDockerImageToCluster(kindClusterName, "crossplane/crossplane:latest"),

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
