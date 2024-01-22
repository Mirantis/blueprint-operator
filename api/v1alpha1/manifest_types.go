package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManifestSpec defines the desired state of Manifest
type ManifestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Url string `json:"url"`

	// This flag tells the controller how to handle the manifest in case of a failure.
	// Valid values are:
	// - None (default) : No-op; No action is triggered on manifest failure
	// - Retry : Manifest is retried in case of failure. For install, the manifest resources are deleted and re-installed.
	//			 For update, the new version of the manifest is applied on top of existing resources.
	FailurePolicy string `json:"failurePolicy"`

	// Timeout for manifest operations as duration string (300s, 10m, 1h, etc)
	// If manifest is not Available after timeout duration, it will be handled by specified FailurePolicy
	// +optional
	Timeout string `json:"timeout"`

	NewChecksum string           `json:"newChecksum,omitempty"`
	Checksum    string           `json:"checksum"`
	Objects     []ManifestObject `json:"objects,omitempty"`
}

// ManifestStatus defines the observed state of Manifest
type ManifestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Status `json:",inline"`
}

// ManifestObject consists of the fields required to update/delete an object
type ManifestObject struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.type",description="Whether the component is running and stable."

// Manifest is the Schema for the manifests API
type Manifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManifestSpec   `json:"spec,omitempty"`
	Status ManifestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManifestList contains a list of Manifest
type ManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Manifest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Manifest{}, &ManifestList{})
}
