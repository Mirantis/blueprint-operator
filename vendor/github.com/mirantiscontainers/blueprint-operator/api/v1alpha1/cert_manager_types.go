package v1alpha1

import (
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

type Issuer struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:Required
	Spec certmanager.IssuerSpec `json:"spec"`
}

type ClusterIssuer struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Spec certmanager.IssuerSpec `json:"spec"`
}

type Certificate struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:Required
	Spec certmanager.CertificateSpec `json:"spec"`
}
