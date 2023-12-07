package bopinstall

import (
	//"context"
	//"fmt"
	"os"
	"testing"
	//"time"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	kindClusterName = "kind-bopinstall"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	cfg, _ := envconf.NewFromFlags()
	testenv = env.NewWithConfig(cfg)
	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),
	)

	testenv.Finish(
		envfuncs.DestroyCluster(kindClusterName),
	)
	os.Exit(testenv.Run(m))
}
