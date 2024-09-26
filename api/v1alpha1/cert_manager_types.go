package v1alpha1

import (
	"fmt"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

// CertManagement defines the desired state of cert-manager resources
type CertManagement struct {
	Issuers        []Issuer        `json:"issuers,omitempty"`
	ClusterIssuers []ClusterIssuer `json:"clusterIssuers,omitempty"`
	Certificates   []Certificate   `json:"certificates,omitempty"`
}

// Validate checks the CertManagement structure and its children
func (c *CertManagement) Validate() error {
	for _, issuer := range c.Issuers {
		if issuer.Name == "" {
			return fmt.Errorf("issuer name cannot be empty")
		}
		if issuer.Namespace == "" {
			return fmt.Errorf("issuer namespace cannot be empty")
		}
	}

	for _, clusterIssuer := range c.ClusterIssuers {
		if clusterIssuer.Name == "" {
			return fmt.Errorf("cluster issuer name cannot be empty")
		}
	}

	for _, certificate := range c.Certificates {
		if certificate.Name == "" {
			return fmt.Errorf("certificate name cannot be empty")
		}
		if certificate.Namespace == "" {
			return fmt.Errorf("certificate namespace cannot be empty")
		}
		if certificate.Spec.IssuerRef.Name == "" {
			return fmt.Errorf("certificate issuer name cannot be empty")
		}
		if certificate.Spec.IssuerRef.Kind == "" {
			return fmt.Errorf("certificate issuer kind cannot be empty")
		}
	}

	return nil
}

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
