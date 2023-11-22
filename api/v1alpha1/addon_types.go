package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AddonSpec defines the desired state of Addon
type AddonSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Name      string        `json:"name"`
	Kind      string        `json:"kind"`
	Enabled   bool          `json:"enabled"`
	Namespace string        `json:"namespace,omitempty"`
	Chart     *ChartInfo    `json:"chart,omitempty"`
	Manifest  *ManifestInfo `json:"manifest,omitempty"`
}

type ChartInfo struct {
	Name    string                        `json:"name"`
	Repo    string                        `json:"repo"`
	Version string                        `json:"version"`
	Set     map[string]intstr.IntOrString `json:"set,omitempty"`
	Values  string                        `json:"values,omitempty"`
}

type ManifestInfo struct {
	URL string `json:"url"`
}

// AddonStatus defines the observed state of Addon
type AddonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Addon is the Schema for the addons API
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AddonList contains a list of Addon
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}
