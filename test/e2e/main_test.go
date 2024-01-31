package e2e

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"

	"github.com/mirantiscontainers/boundless-operator/test/e2e/funcs"
)

var (
	testenv env.Environment
)

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("test-cluster", 16)

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),

		// add boundless types to scheme
		funcs.AddBoundlessTypeToScheme(),

		// install boundless operator
		funcs.InstallBoundlessOperator(),
	)

	testenv.BeforeEachFeature()

	testenv.Finish(
	//envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}
