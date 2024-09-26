package types

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// TestBlueprintValidateAPIVersion tests the validation of a Blueprint's APIVersion
func TestBlueprintValidateAPIVersion(t *testing.T) {
	tests := map[string]struct {
		version string
		want    types.GomegaMatcher
	}{
		"valid version": {version: "boundless.mirantis.com/v1alpha1", want: BeNil()},
		"empty version": {version: "", want: Equal(fmt.Errorf("apiVersion field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			blueprint := Blueprint{
				APIVersion: tc.version,
				Kind:       "Blueprint", // This is required for Validate() to work but not tested here
			}
			actual := blueprint.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestBlueprintValidateKind tests the validation of a Blueprint's Kind
func TestBlueprintValidateKind(t *testing.T) {
	tests := map[string]struct {
		kind string
		want types.GomegaMatcher
	}{
		"valid kind": {kind: blueprintKinds[0], want: BeNil()},
		"wrong kind": {kind: "Tacos", want: Equal(fmt.Errorf("invalid cluster kind: Tacos"))},
		"empty kind": {kind: "", want: Equal(fmt.Errorf("kind field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			blueprint := Blueprint{
				APIVersion: "boundless.mirantis.com/v1alpha1", // This is required for Validate() to work but not tested here
				Kind:       tc.kind,
			}
			actual := blueprint.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestKubernetesValidateProvider tests the validation of a Kubernete's provider
func TestKubernetesValidateProvider(t *testing.T) {
	tests := map[string]struct {
		provider string
		want     types.GomegaMatcher
	}{
		"valid provider": {provider: providerKinds[0], want: BeNil()},
		"wrong provider": {provider: "Tacos", want: Equal(fmt.Errorf("invalid kubernetes.provider: Tacos"))},
		"empty provider": {provider: "", want: Equal(fmt.Errorf("kubernetes.provider field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			kubernetes := Kubernetes{
				Provider: tc.provider,
			}
			actual := kubernetes.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestKubernetesValidateVersion tests the validation of a Kubernete's version
func TestKubernetesValidateVersion(t *testing.T) {
	tests := map[string]struct {
		version string
		want    types.GomegaMatcher
	}{
		"semver version":       {version: "1.2.3", want: BeNil()},
		"semver + k0s version": {version: "1.2.3+k0s.0", want: BeNil()},
		"Invaklid k0s version": {version: "1.2.3+k0.0", want: Equal(fmt.Errorf("invalid kubernetes.version: 1.2.3+k0.0"))},
		"wrong version":        {version: "Tacos", want: Equal(fmt.Errorf("invalid kubernetes.version: Tacos"))},
		"no version":           {version: "", want: BeNil()},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			kubernetes := Kubernetes{
				Provider: providerKinds[0], // This is required for Validate() to work but not tested here
				Version:  tc.version,
			}
			actual := kubernetes.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestAddonValidateName tests the validation of a Addon's name
func TestAddonValidateName(t *testing.T) {
	tests := map[string]struct {
		name string
		want types.GomegaMatcher
	}{
		"valid name": {name: "Bob", want: BeNil()},
		"no name":    {name: "", want: Equal(fmt.Errorf("addons.name field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			addon := Addon{
				Name: tc.name,
				Kind: "manifest", // This is required for Validate() to work but not tested here
				Manifest: &ManifestInfo{
					URL: "https://charts.bitnami.com/bitnami", // This is required for Validate() to work but not tested here
				},
			}
			actual := addon.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)
		})
	}
}

// TestAddonValidateKind tests the validation of a Addon's kind
func TestAddonValidateKind(t *testing.T) {
	tests := map[string]struct {
		kind string
		want types.GomegaMatcher
	}{
		"valid lowercase kind": {kind: "manifest", want: BeNil()},
		"valid uppercase kind": {kind: "Manifest", want: BeNil()},
		"no kind":              {kind: "", want: Equal(fmt.Errorf("addons.kind field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			addon := Addon{
				Name: "Bob", // This is required for Validate() to work but not tested here
				Kind: tc.kind,
				Manifest: &ManifestInfo{
					URL: "https://charts.bitnami.com/bitnami", // This is required for Validate() to work but not tested here
				},
			}
			actual := addon.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestAddonValidateKindCount tests the proper addon structure when a kind is specified
func TestAddonValidateKindCount(t *testing.T) {
	tests := map[string]struct {
		kind     string
		manifest bool
		chart    bool
		want     types.GomegaMatcher
	}{
		"correct manifest":       {kind: "manifest", manifest: true, chart: false, want: BeNil()},
		"correct chart":          {kind: "chart", manifest: false, chart: true, want: BeNil()},
		"no addon structs":       {kind: "manifest", manifest: false, chart: false, want: Equal(fmt.Errorf("Super Cool Addon: addon must contain a chart or manifest"))},
		"multiple addon structs": {kind: "manifest", manifest: true, chart: true, want: Equal(fmt.Errorf("Super Cool Addon: addon cannot contain both a chart and a manifest"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			addon := Addon{
				Name: "Super Cool Addon", // This is required for Validate() to work but not tested here
				Kind: tc.kind,
			}
			if tc.manifest {
				addon.Manifest = &ManifestInfo{
					URL: "https://charts.bitnami.com/bitnami", // This is required for ManifestInfo.Validate() to work but not tested here
				}
			}
			if tc.chart {
				addon.Chart = &ChartInfo{
					Name:    "Fred",                               // This is required for ChartInfo.Validate() to work but not tested here
					Repo:    "https://charts.bitnami.com/bitnami", // This is required for ChartInfo.Validate() to work but not tested here
					Version: "1.2.3",                              // This is required for ChartInfo.Validate() to work but not tested here
				}
			}
			actual := addon.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestChartInfoValidateName tests the validation of a ChartInfo's name
func TestChartInfoValidateName(t *testing.T) {
	tests := map[string]struct {
		name string
		want types.GomegaMatcher
	}{
		"valid name": {name: "Fred", want: BeNil()},
		"no name":    {name: "", want: Equal(fmt.Errorf("chart.name field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			chart := ChartInfo{
				Name:    tc.name,
				Repo:    "https://charts.bitnami.com/bitnami", // This is required for Validate() to work but not tested here
				Version: "1.2.3",                              // This is required for Validate() to work but not tested here
			}
			actual := chart.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestChartInfoValidateRepo tests the validation of a ChartInfo's repo
func TestChartInfoValidateRepo(t *testing.T) {
	tests := map[string]struct {
		repo string
		want types.GomegaMatcher
	}{
		"valid repo": {repo: "https://charts.bitnami.com/bitnami", want: BeNil()},
		"no repo":    {repo: "", want: Equal(fmt.Errorf("chart.repo field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			chart := ChartInfo{
				Name:    "George", // This is required for Validate() to work but not tested here
				Repo:    tc.repo,
				Version: "1.2.3", // This is required for Validate() to work but not tested here
			}
			actual := chart.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestChartInfoValidateVersion tests the validation of a ChartInfo's version
func TestChartInfoValidateVersion(t *testing.T) {
	tests := map[string]struct {
		version string
		want    types.GomegaMatcher
	}{
		"valid version": {version: "1.2.3", want: BeNil()},
		"no version":    {version: "", want: Equal(fmt.Errorf("chart.version field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			chart := ChartInfo{
				Name:    "George",                             // This is required for Validate() to work but not tested here
				Repo:    "https://charts.bitnami.com/bitnami", // This is required for Validate() to work but not tested here
				Version: tc.version,
			}

			actual := chart.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}
