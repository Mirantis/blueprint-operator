package controllers

import "github.com/mirantis/boundless-operator/pkg/helm"

var NginxIngressHelmChart = helm.Chart{
	Name:    "ingress-nginx",
	Version: "4.7.1",
	Repo:    "https://kubernetes.github.io/ingress-nginx",
}

var KongIngressHelmChart = helm.Chart{
	Name:    "kong",
	Version: "0.4.0",
	Repo:    "https://charts.konghq.com",
}
