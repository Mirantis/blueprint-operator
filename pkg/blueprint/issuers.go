package blueprint

import (
	"fmt"

	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Issuer struct {
	certmanager.Issuer `json:",inline"`
}

func (i *Issuer) GetComponentName() string {
	return fmt.Sprintf("%s/%s", i.Namespace, i.Name)
}

func (i *Issuer) GetComponentNamespace() string {
	return i.Namespace
}

func (i *Issuer) GetObject() client.Object {
	return &(i.Issuer)
}

func (i *Issuer) SetObject(obj client.Object) error {
	issuer, ok := obj.(*certmanager.Issuer)
	if !ok {
		return fmt.Errorf("object is not an Issuer")
	}
	i.Issuer = *issuer
	return nil
}

type IssuerList struct {
	certmanager.IssuerList `json:",inline"`
}

func (l *IssuerList) GetItems() []*Issuer {
	items := make([]*Issuer, len(l.Items))
	for i, item := range l.Items {
		items[i] = &Issuer{item}
	}
	return items
}

func (l *IssuerList) GetObjectList() client.ObjectList {
	return &l.IssuerList
}

type ClusterIssuer struct {
	certmanager.ClusterIssuer `json:",inline"`
}

func (i *ClusterIssuer) GetComponentName() string {
	return i.Name
}

func (i *ClusterIssuer) GetComponentNamespace() string {
	return ""
}

func (i *ClusterIssuer) GetObject() client.Object {
	return &(i.ClusterIssuer)
}

func (i *ClusterIssuer) SetObject(obj client.Object) error {
	issuer, ok := obj.(*certmanager.ClusterIssuer)
	if !ok {
		return fmt.Errorf("object is not a ClusterIssuer")
	}
	i.ClusterIssuer = *issuer
	return nil
}

type ClusterIssuerList struct {
	certmanager.ClusterIssuerList `json:",inline"`
}

func (l *ClusterIssuerList) GetItems() []*ClusterIssuer {
	items := make([]*ClusterIssuer, len(l.Items))
	for i, item := range l.Items {
		items[i] = &ClusterIssuer{item}
	}
	return items
}

func (l *ClusterIssuerList) GetObjectList() client.ObjectList {
	return &l.ClusterIssuerList
}
