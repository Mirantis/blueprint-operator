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

// StatusType is a type of condition that may apply to a particular component.
type StatusType string

const (
	// TypeComponentAvailable indicates that the component is healthy.
	TypeComponentAvailable StatusType = "Available"

	// TypeComponentProgressing means that the component is in the process of being installed or upgraded.
	TypeComponentProgressing StatusType = "Progressing"

	// TypeComponentDegraded means the component is not operating as desired and user action is required.
	TypeComponentDegraded StatusType = "Degraded"

	// TypeComponentReady indicates that the component is healthy and ready.it is identical to Available.
	TypeComponentReady StatusType = "Ready"

	// TypeComponentUnhealthy indicates the component is not functioning as intended.
	TypeComponentUnhealthy StatusType = "Unhealthy"
)

type Status struct {
	// The type of condition. May be Available, Progressing, or Degraded.
	Type StatusType `json:"type"`

	// The timestamp representing the start time for the current status.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// A brief reason explaining the condition.
	Reason string `json:"reason,omitempty"`

	// Optionally, a detailed message providing additional context.
	Message string `json:"message,omitempty"`
}

// AddonStatus defines the observed state of Addon
type AddonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.type",description="Whether the component is running and stable."

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
