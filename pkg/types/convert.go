package types

import (
	"github.com/k0sproject/dig"
	v1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"sigs.k8s.io/yaml"
)

const apiVersion = "blueprint.mirantis.com/v1alpha1"
const apiVersionK0s = "k0sctl.k0sproject.io/v1beta1"

func ParseK0sCluster(data []byte) (K0sCluster, error) {
	var cluster K0sCluster
	err := yaml.Unmarshal(data, &cluster)
	if err != nil {
		return K0sCluster{}, err
	}
	return cluster, nil
}

func ParseBoundlessCluster(data []byte) (Blueprint, error) {
	var cluster Blueprint
	err := yaml.Unmarshal(data, &cluster)
	if err != nil {
		return Blueprint{}, err
	}

	return cluster, nil
}

func ParseCoreComponentManifests(data []byte) (v1.HelmChart, error) {
	var helmChart v1.HelmChart
	err := yaml.Unmarshal(data, &helmChart)
	if err != nil {
		return v1.HelmChart{}, err
	}

	return helmChart, nil
}

func ConvertToK0s(cluster *Blueprint) K0sCluster {
	return K0sCluster{
		APIVersion: apiVersionK0s,
		Kind:       "Cluster",
		Metadata: Metadata{
			Name: cluster.Metadata.Name,
		},
		Spec: K0sClusterSpec{
			Hosts: cluster.Spec.Kubernetes.Infra.Hosts,
			K0S: K0s{
				Version:       cluster.Spec.Kubernetes.Version,
				DynamicConfig: digBool(cluster.Spec.Kubernetes.Config, "dynamicConfig"),
				Config:        cluster.Spec.Kubernetes.Config,
			},
		},
	}
}

func ConvertToClusterWithK0s(k0s K0sCluster, components Components) Blueprint {
	return Blueprint{
		APIVersion: apiVersion,
		Kind:       "Blueprint",
		Metadata: Metadata{
			Name: k0s.Metadata.Name,
		},
		Spec: BlueprintSpec{
			Version: "latest",
			Kubernetes: &Kubernetes{
				Provider: "k0s",
				Version:  k0s.Spec.K0S.Version,
				Config:   k0s.Spec.K0S.Config,
				Infra: &Infra{
					Hosts: k0s.Spec.Hosts,
				},
			},
			Components: components,
		},
	}
}

func ConvertToClusterWithKind(name string, components Components) Blueprint {
	return Blueprint{
		APIVersion: apiVersion,
		Kind:       "Blueprint",
		Metadata: Metadata{
			Name: name,
		},
		Spec: BlueprintSpec{
			Version: "latest",
			Kubernetes: &Kubernetes{
				Provider: "kind",
			},
			Components: components,
		},
	}
}

func DigToString(m dig.Mapping, keys ...string) string {
	val := m.Dig(keys...)
	if val == nil {
		return ""
	}
	return val.(string)
}

func digBool(m dig.Mapping, keys ...string) bool {
	val := m.Dig(keys...)
	if val == nil {
		return false
	}
	return val.(bool)
}
