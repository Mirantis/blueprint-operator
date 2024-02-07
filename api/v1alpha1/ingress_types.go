package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressSpec defines the desired state of Ingress
type IngressSpec struct {

	// Enabled is a flag to enable/disable the ingress
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Required
	Provider string `json:"provider"`

	Config string `json:"config,omitempty"`
}

// IngressStatus defines the observed state of Ingress
type IngressStatus struct {
	IngressReady bool `json:"ingressReady"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Ingress is the Schema for the ingresses API
type Ingress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressSpec   `json:"spec,omitempty"`
	Status IngressStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IngressList contains a list of Ingress
type IngressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ingress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ingress{}, &IngressList{})
}
