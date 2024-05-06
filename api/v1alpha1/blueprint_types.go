package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlueprintSpec defines the desired state of Blueprint
type BlueprintSpec struct {
	// Components contains all the components that should be installed
	Components Component `json:"components,omitempty"`
	// Resources contains all object resources that should be installed
	Resources Resources `json:"resources,omitempty"`
}

// Component defines the addons components that should be installed
type Component struct {
	Addons []AddonSpec `json:"addons,omitempty"`
}

// Resources defines the desired state of kubernetes resources that should be managed by BOP
type Resources struct {
	CertManagement CertManagement `json:"certManagement,omitempty"`
}

// CertManagement defines the desired state of cert-manager resources
type CertManagement struct {
	Issuers        []Issuer        `json:"issuers,omitempty"`
	ClusterIssuers []ClusterIssuer `json:"clusterIssuers,omitempty"`
	Certificates   []Certificate   `json:"certificates,omitempty"`
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
