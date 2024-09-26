package v1alpha1

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"

	"github.com/k0sproject/dig"

	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
)

var (
	// ProviderK0S is the name of the k0s distro
	ProviderK0s = "k0s"
	// ProviderKind is the name of the kind distro
	ProviderKind = "kind"
	// ProviderExisting is the name of an existing unofficial distro
	ProviderExisting = "existing"
)

type Kubernetes struct {
	Provider string `yaml:"provider" json:"provider"`
	Version  string `yaml:"version,omitempty" json:"version,omitempty"`
	// Config     dig.Mapping `yaml:"config,omitempty" json:"config,omitempty"`
	Infra      *Infra `yaml:"infra,omitempty" json:"infra,omitempty"`
	KubeConfig string `yaml:"kubeconfig,omitempty" json:"kubeConfig,omitempty"`
}

var providerKinds = []string{ProviderExisting, ProviderKind, ProviderK0s}

// Validate checks the Kubernetes structure and its children
func (k *Kubernetes) Validate() error {
	// Provider checks
	if k.Provider == "" {
		return fmt.Errorf("kubernetes.provider field cannot be left blank")
	}
	if !slices.Contains(providerKinds, k.Provider) {
		return fmt.Errorf("invalid kubernetes.provider: %s", k.Provider)
	}

	// Version checks
	// The version can be left empty, but if it's not, it must be a valid k0s semver
	if k.Version != "" {
		re, _ := regexp.Compile(consts.K0sSemverRegex)
		if !re.MatchString(k.Version) {
			return fmt.Errorf("invalid kubernetes.version: %s", k.Version)
		}
	}

	// Infra checks
	if k.Infra != nil {
		if err := k.Infra.Validate(); err != nil {
			return err
		}
	}

	// KubeConfig checks
	if k.KubeConfig != "" {
		if _, err := os.Stat(k.KubeConfig); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("kubernetes.kubeConfig file %q does not exist: %s", k.KubeConfig, err)
		}
	}

	return nil
}

type Infra struct {
	Hosts []Host `yaml:"hosts" json:"hosts"`
}

// Validate checks the Infra structure and its children
func (i *Infra) Validate() error {

	// Host checks
	for _, host := range i.Hosts {
		if err := host.Validate(); err != nil {
			return err
		}
	}

	return nil
}

type Host struct {
	SSH          *SSHHost   `yaml:"ssh,omitempty" json:"ssh,omitempty"`
	LocalHost    *LocalHost `yaml:"localhost,omitempty" json:"localHost,omitempty"`
	Role         string     `yaml:"role" json:"role"`
	InstallFlags []string   `yaml:"installFlags,omitempty" json:"installFlags,omitempty"`
}

var nodeRoles = []string{"single", "controller", "worker", "controller+worker"}

// Validate checks the Host structure and its children
func (h *Host) Validate() error {

	// SSH checks
	if h.SSH != nil {
		if err := h.SSH.Validate(); err != nil {
			return err
		}
	}

	// Localhost checks
	if h.LocalHost != nil {
		if err := h.LocalHost.Validate(); err != nil {
			return err
		}
	}

	// Role checks
	if h.Role == "" {
		return fmt.Errorf("hosts.role field cannot be left blank")
	}
	if !slices.Contains(nodeRoles, h.Role) {
		return fmt.Errorf("invalid hosts.role: %s\nValid hosts.role values: %s", h.Role, nodeRoles)
	}

	return nil
}

type SSHHost struct {
	Address string `yaml:"address" json:"address"`
	KeyPath string `yaml:"keyPath" json:"keyPath"`
	Port    int    `yaml:"port" json:"port"`
	User    string `yaml:"user" json:"user"`
}

// Validate checks the SSHHost structure and its children
func (sh *SSHHost) Validate() error {

	// Address checks
	if sh.Address == "" {
		return fmt.Errorf("hosts.ssh.address field cannot be left empty")
	}
	// This regex is for either valid hostnames or ip addresses
	re, _ := regexp.Compile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)
	if !re.MatchString(sh.Address) {
		return fmt.Errorf("invalid hosts.ssh.address: %s", sh.Address)
	}

	// KeyPath checks
	if sh.KeyPath == "" {
		return fmt.Errorf("hosts.ssh.keypath field cannot be left empty")
	}
	if _, err := os.Stat(sh.KeyPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("hosts.ssh.keypath does not exist: %s", sh.KeyPath)
	}

	// Port checks
	if sh.Port <= 0 || sh.Port > 65535 {
		return fmt.Errorf("hosts.ssh.port outside of valid range 0-65535")
	}

	// User checks
	if sh.User == "" {
		return fmt.Errorf("hosts.ssh.user cannot be left empty")
	}

	return nil
}

type LocalHost struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// Validate checks the LocalHost structure and its children
func (l *LocalHost) Validate() error {
	// This is just a placeholder for now
	return nil
}

type K0sCluster struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       string         `yaml:"kind" json:"kind"`
	Metadata   Metadata       `yaml:"metadata" json:"metadata"`
	Spec       K0sClusterSpec `yaml:"spec" json:"spec"`
}

type K0sClusterSpec struct {
	Hosts []Host `yaml:"hosts" json:"hosts"`
	K0S   K0s    `yaml:"k0s" json:"k0s"`
}

type K0s struct {
	Version       string      `yaml:"version" json:"version"`
	DynamicConfig bool        `yaml:"dynamicConfig" json:"dynamicConfig"`
	Config        dig.Mapping `yaml:"config,omitempty" json:"config"`
}

type Metadata struct {
	Name string `yaml:"name" json:"name"`
}

// Validate checks the Metadata structure and its children
func (m *Metadata) Validate() error {
	// This is just a placeholder for now

	return nil
}
