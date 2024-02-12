package e2e

import (
	"flag"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

type Environment struct {
	img *string
	env.Environment
}

// NewEnvironmentFromFlags creates a new e2e test configuration, setting up the flags, but
// not parsing them yet, which is left to the caller to do.
func NewEnvironmentFromFlags() Environment {
	c := Environment{}
	c.img = flag.String("img", defaultOperatorImage, "operator image to use for the test suite")
	return c
}

func (e *Environment) GetOperatorImage() string {
	return *e.img
}

// SetEnvironment sets the environment to be used by the e2e test configuration.
func (e *Environment) SetEnvironment(env env.Environment) {
	e.Environment = env
}
