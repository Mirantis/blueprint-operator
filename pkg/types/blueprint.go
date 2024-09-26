package types

import (
	"fmt"
	"slices"

	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
)

var blueprintKinds = []string{"Blueprint"}

type Blueprint struct {
	APIVersion string                 `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                 `yaml:"kind" json:"kind"`
	Metadata   v1alpha1.Metadata      `yaml:"metadata" json:"metadata"`
	Spec       v1alpha1.BlueprintSpec `yaml:"spec" json:"spec"`
}

// Validate checks the Blueprint structure and its children
func (b *Blueprint) Validate() error {
	// APIVersion checks
	if b.APIVersion == "" {
		return fmt.Errorf("apiVersion field cannot be left blank")
	}

	// Kind checks
	if b.Kind == "" {
		return fmt.Errorf("kind field cannot be left blank")
	}
	if !slices.Contains(blueprintKinds, b.Kind) {
		return fmt.Errorf("invalid cluster kind: %s", b.Kind)
	}

	// Metadata checks
	if err := b.Metadata.Validate(); err != nil {
		return err
	}

	// Spec checks
	if err := b.Spec.Validate(); err != nil {
		return err
	}

	return nil
}
