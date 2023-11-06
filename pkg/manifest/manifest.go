package manifest

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	policy_v1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

type ManifestController struct {
	client client.Client
	logger logr.Logger
}

func NewManifestController(client client.Client, logger logr.Logger) *ManifestController {
	return &ManifestController{
		client: client,
		logger: logger,
	}
}

func (mc *ManifestController) Deserialize(data []byte) (*client.Object, error) {
	apiextensionsv1.AddToScheme(scheme.Scheme)
	apiextensionsv1beta1.AddToScheme(scheme.Scheme)
	decoder := scheme.Codecs.UniversalDeserializer()

	for _, obj := range strings.Split(string(data), "---") {
		runtimeObject, groupVersionKind, err := decoder.Decode([]byte(obj), nil, nil)
		if err != nil {
			return nil, err
		}

		mc.logger.Info("Decode details", "runtimeObject", runtimeObject, "groupVersionKind", groupVersionKind)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		mc.logger.Info("The object recvd is:", "Kind", groupVersionKind.Kind)

		switch groupVersionKind.Kind {
		case "Namespace":
			// @TODO: create the namespace if it doesn't exist
			namespaceObj := convertToNamespaceObject(runtimeObject)
			mc.logger.Info("Creating namespace", "Namespace", namespaceObj.Name)
			err = mc.client.Create(ctx, namespaceObj)
			if err != nil {
				return nil, err
			}
			mc.logger.Info("Namespace created successfully:", "Namespace", namespaceObj.Name)
		case "Service":
			// @TODO: create the service if it doesn't exist
			serviceObj := convertToServiceObject(runtimeObject)
			mc.logger.Info("Creating service", "Service", serviceObj.Name)
			mc.logger.Info("Creating service", "Spec", serviceObj.Spec)
			if serviceObj.Namespace == "" {
				serviceObj.Namespace = "default"
			}
			err = mc.client.Create(ctx, serviceObj)
			if err != nil {
				mc.logger.Info("Failed to create service:", "Error", err)
				return nil, err
			}
			mc.logger.Info("Service created successfully:", "Service", serviceObj.Name)

		case "Deployment":
			// @TODO: create the deployment if it doesn't exist
			deploymentObj := convertToDeploymentObject(runtimeObject)
			mc.logger.Info("Creating deployment", "Name", deploymentObj.Name)
			mc.logger.Info("Creating deployment", "Spec", deploymentObj.Spec)
			if deploymentObj.Namespace == "" {
				deploymentObj.Namespace = "default"
			}
			err = mc.client.Create(ctx, deploymentObj)
			if err != nil {
				mc.logger.Info("Failed to create deployment:", "Error", err)
				return nil, err
			}
			mc.logger.Info("Deployment created successfully:", "Deployment", deploymentObj.Name)

		case "DaemonSet":
			// @TODO: create the daemonSet if it doesn't exist
			daemonsetObj := convertToDaemonsetObject(runtimeObject)
			mc.logger.Info("Creating daemonset", "Name", daemonsetObj.Name)
			mc.logger.Info("Creating daemonset", "Spec", daemonsetObj.Spec)
			if daemonsetObj.Namespace == "" {
				daemonsetObj.Namespace = "default"
			}
			err = mc.client.Create(ctx, daemonsetObj)
			if err != nil {
				mc.logger.Info("Failed to create daemonset:", "Error", err)
				return nil, err
			}
			mc.logger.Info("daemonset created successfully:", "Daemonset", daemonsetObj.Name)

		case "PodDisruptionBudget":
			pdbObj := convertToPodDisruptionBudget(runtimeObject)
			mc.logger.Info("Creating pod disruption budget", "Name", pdbObj.Name)
			mc.logger.Info("Creating pdb", "Spec", pdbObj.Spec)
			if pdbObj.Namespace == "" {
				pdbObj.Namespace = "default"
			}
			err = mc.client.Create(ctx, pdbObj)
			if err != nil {
				mc.logger.Info("Failed to create pod disruption budget:", "Error", err)
				return nil, err
			}
			mc.logger.Info("Pod disruption budget created successfully:", "PodDisruptionBudget", pdbObj.Name)

		case "ServiceAccount":
			serviceAccObj := convertToServiceAccount(runtimeObject)
			mc.logger.Info("Creating service account", "Name", serviceAccObj.Name)
			if serviceAccObj.Namespace == "" {
				serviceAccObj.Namespace = "default"
			}
			err = mc.client.Create(ctx, serviceAccObj)
			if err != nil {
				mc.logger.Info("Failed to create service account:", "Error", err)
				return nil, err
			}
			mc.logger.Info("service account created successfully:", "ServiceAccount", serviceAccObj.Name)

		case "CustomResourceDefinition":
			crdObj := convertToCRDObject(runtimeObject)
			mc.logger.Info("Creating CRD", "Name", crdObj.Name)
			/*if crdObj.Namespace == "" {
			    crdObj.Namespace = "default"
			}*/
			err = mc.client.Create(ctx, crdObj)
			if err != nil {
				mc.logger.Info("Failed to create crd:", "Error", err)
				return nil, err
			}
			mc.logger.Info("crd created successfully:", "CRD", crdObj.Name)

		default:
			mc.logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
		}
	}

	return nil, nil
}

func convertToNamespaceObject(obj runtime.Object) *core_v1.Namespace {
	myobj := obj.(*core_v1.Namespace)
	return myobj
}

func convertToServiceObject(obj runtime.Object) *core_v1.Service {
	myobj := obj.(*core_v1.Service)
	return myobj
}

func convertToDeploymentObject(obj runtime.Object) *apps_v1.Deployment {
	myobj := obj.(*apps_v1.Deployment)
	return myobj
}

func convertToPodDisruptionBudget(obj runtime.Object) *policy_v1.PodDisruptionBudget {
	myobj := obj.(*policy_v1.PodDisruptionBudget)
	return myobj
}

func convertToServiceAccount(obj runtime.Object) *core_v1.ServiceAccount {
	myobj := obj.(*core_v1.ServiceAccount)
	return myobj
}

func convertToCRDObject(obj runtime.Object) *apiextensionsv1.CustomResourceDefinition {
	myobj := obj.(*apiextensionsv1.CustomResourceDefinition)
	return myobj
}

func convertToDaemonsetObject(obj runtime.Object) *apps_v1.DaemonSet {
	myobj := obj.(*apps_v1.DaemonSet)
	return myobj
}
