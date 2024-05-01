package v1alpha1

import (
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mirantiscontainers/boundless-operator/pkg/blueprint"
)

// CASpec defines the desired state of Certificate Authorities
type CASpec struct {
	Issuers        []Issuer        `json:"issuers,omitempty"`
	ClusterIssuers []ClusterIssuer `json:"clusterIssuers,omitempty"`
}

type Issuer struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:Required
	Spec certmanager.IssuerSpec `json:"spec"`
}

func (i *Issuer) GetName() string {
	return i.Name
}

func (i *Issuer) GetNamespace() string {
	return i.Namespace
}

func (i *Issuer) SetNamespace(namespace string) {
	i.Namespace = namespace
}

func (i *Issuer) IsClusterScoped() bool {
	return false
}

func (i *Issuer) MakeObject() *blueprint.Issuer {
	return &blueprint.Issuer{Issuer: certmanager.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.Name,
			Namespace: i.Namespace,
		},
		Spec: i.Spec,
	}}
}

type ClusterIssuer struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Spec certmanager.IssuerSpec `json:"spec"`
}

func (ci *ClusterIssuer) GetName() string {
	return ci.Name
}

func (ci *ClusterIssuer) GetNamespace() string {
	return ""
}

func (ci *ClusterIssuer) SetNamespace(_ string) {}

func (ci *ClusterIssuer) IsClusterScoped() bool {
	return true
}

func (ci *ClusterIssuer) MakeObject() *blueprint.ClusterIssuer {
	return &blueprint.ClusterIssuer{ClusterIssuer: certmanager.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name: ci.Name,
		},
		Spec: ci.Spec,
	}}
}
