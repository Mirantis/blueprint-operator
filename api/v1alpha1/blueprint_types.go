package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlueprintSpec defines the desired state of Blueprint
type BlueprintSpec struct {
	Version    string      `yaml:"version" json:"version"`
	Kubernetes *Kubernetes `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`
	// Components contains all the components that should be installed
	Components Components `yaml:"components" json:"components"`
	// Resources contains all object resources that should be installed
	Resources *Resources `yaml:"resources,omitempty" json:"resources,omitempty"`
}

// Validate checks the BlueprintSpec structure and its children
func (bs *BlueprintSpec) Validate() error {

	// Kubernetes checks
	if bs.Kubernetes != nil {
		if err := bs.Kubernetes.Validate(); err != nil {
			return err
		}
	}

	// Components checks
	if err := bs.Components.Validate(); err != nil {
		return err
	}

	// Resources checks
	if bs.Resources != nil {
		if err := bs.Resources.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Component defines the addons components that should be installed
type Components struct {
	Addons []Addon `yaml:"addons,omitempty" json:"addons,omitempty"`
}

// Validate checks the Components structure and its children
func (c *Components) Validate() error {
	// TODO Core components aren't checked because they will likely be removed/moved to MKE4

	// Addon checks
	for _, addon := range c.Addons {
		if err := addon.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Resources defines the desired state of kubernetes resources that should be managed by BOP
type Resources struct {
	CertManagement CertManagement `yaml:"certManagement,omitempty" json:"certManagement,omitempty"`
}

// Validate checks the Resources structure and its children
func (r *Resources) Validate() error {
	if err := r.CertManagement.Validate(); err != nil {
		return err
	}

	return nil
}

// BlueprintStatus defines the observed state of Blueprint
type BlueprintStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Blueprint is the Schema for the blueprints API
type Blueprint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BlueprintSpec   `json:"spec,omitempty"`
	Status BlueprintStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BlueprintList contains a list of Blueprint
type BlueprintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Blueprint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Blueprint{}, &BlueprintList{})
}
