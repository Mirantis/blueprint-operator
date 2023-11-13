package manifest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	adm_v1 "k8s.io/api/admissionregistration/v1"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	policy_v1 "k8s.io/api/policy/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func (mc *ManifestController) CreateManifest(namespace, name, url string) error {

	var client http.Client

	// Run http get request to fetch the contents of the manifest file
	resp, err := client.Get(url)
	if err != nil {
		mc.logger.Error(err, "failed to run Unable to read response")
		return err
	}

	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			mc.logger.Error(err, "failed to read http response body")
			return err
		}

	} else {
		mc.logger.Error(err, "failure in http get request", "ResponseCode", resp.StatusCode)
		return err
	}

	return mc.createOrUpdateManifest(namespace, name, url, bodyBytes)

}

func (mc *ManifestController) createOrUpdateManifest(namespace, name, url string, bodyBytes []byte) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := mc.getExistingManifest(ctx, namespace, name)
	if err != nil {
		return err
	}

	if existing != nil {
		// ToDo : add code for update
		mc.logger.Info("manifest crd already exists", "Manifest", existing)
		return nil

	} else {
		mc.logger.Info("manifest crd does not exist, creating", "ManifestName", name, "Namespace", namespace)

		// Deserialize the manifest contents and fetch all the objects
		_, err := mc.CreateManifestCRD(namespace, name, url, bodyBytes)

		if err != nil {
			mc.logger.Error(err, "failed to create manifest CRD")
			return err
		}
	}

	return nil
}

func (mc *ManifestController) getExistingManifest(ctx context.Context, namespace, name string) (*boundlessv1alpha1.Manifest, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existing := &boundlessv1alpha1.Manifest{}
	err := mc.client.Get(ctx, key, existing)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			mc.logger.Info("manifest does not exist", "Namespace", namespace, "ManifestName", name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to get existing manifest: %w", err)
		}
	}
	return existing, nil
}

func (mc *ManifestController) CreateManifestCRD(namespace, name, url string, data []byte) ([]*runtime.Object, error) {
	apiextensionsv1.AddToScheme(scheme.Scheme)
	apiextensionsv1beta1.AddToScheme(scheme.Scheme)
	adm_v1.AddToScheme(scheme.Scheme)
	apps_v1.AddToScheme(scheme.Scheme)
	core_v1.AddToScheme(scheme.Scheme)
	policy_v1.AddToScheme(scheme.Scheme)
	rbac_v1.AddToScheme(scheme.Scheme)

	decoder := scheme.Codecs.UniversalDeserializer()

	for _, obj := range strings.Split(string(data), "---") {
		if obj != "" {
			runtimeObject, groupVersionKind, err := decoder.Decode([]byte(obj), nil, nil)
			if err != nil {
				mc.logger.Info("Failed to decode yaml:", "Error", err)
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
				err := mc.client.Create(ctx, namespaceObj)
				if err != nil {
					if strings.Contains(err.Error(), "already exists") {
						mc.logger.Info("Namespace already exists:", "Namespace", namespaceObj.Name)
						return nil, nil
					}
					return nil, err
				}
				mc.logger.Info("Namespace created successfully:", "Namespace", namespaceObj.Name)

			case "Service":
				// @TODO: create the service if it doesn't exist
				serviceObj := convertToServiceObject(runtimeObject)
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
				if serviceAccObj.Namespace == "" {
					serviceAccObj.Namespace = "default"
				}
				err = mc.client.Create(ctx, serviceAccObj)
				if err != nil {
					mc.logger.Info("Failed to create service account:", "Error", err)
					return nil, err
				}
				mc.logger.Info("service account created successfully:", "ServiceAccount", serviceAccObj.Name)

			case "Role":
				roleObj := convertToRoleObject(runtimeObject)
				if roleObj.Namespace == "" {
					roleObj.Namespace = "default"
				}
				err = mc.client.Create(ctx, roleObj)
				if err != nil {
					mc.logger.Info("Failed to create role:", "Error", err)
					return nil, err
				}
				mc.logger.Info("Role created successfully:", "Role", roleObj.Name)

			case "ClusterRole":
				clusterRoleObj := convertToClusterRoleObject(runtimeObject)
				if clusterRoleObj.Namespace == "" {
					clusterRoleObj.Namespace = "default"
				}
				err = mc.client.Create(ctx, clusterRoleObj)
				if err != nil {
					mc.logger.Info("Failed to create clusterrole:", "Error", err)
					return nil, err
				}
				mc.logger.Info("ClusterRole created successfully:", "Role", clusterRoleObj.Name)

			case "Secret":
				secretObj := convertToSecretObject(runtimeObject)
				if secretObj.Namespace == "" {
					secretObj.Namespace = "default"
				}
				err = mc.client.Create(ctx, secretObj)
				if err != nil {
					mc.logger.Info("Failed to create secret:", "Error", err)
					return nil, err
				}
				mc.logger.Info("secret created successfully:", "Secret", secretObj.Name)

			case "RoleBinding":
				roleBindingObj := convertToRoleBindingObject(runtimeObject)
				if roleBindingObj.Namespace == "" {
					roleBindingObj.Namespace = "default"
				}
				err = mc.client.Create(ctx, roleBindingObj)
				if err != nil {
					mc.logger.Info("Failed to create role binding:", "Error", err)
					return nil, err
				}
				mc.logger.Info("role binding created successfully:", "RoleBinding", roleBindingObj.Name)

			case "ClusterRoleBinding":
				clusterRoleBindingObj := convertToClusterRoleBindingObject(runtimeObject)
				mc.logger.Info("Creating cluster role binding", "Name", clusterRoleBindingObj.Name)

				err = mc.client.Create(ctx, clusterRoleBindingObj)
				if err != nil {
					mc.logger.Info("Failed to create cluster role binding:", "Error", err)
					return nil, err
				}
				mc.logger.Info("cluster role binding created successfully:", "ClusterRoleBinding", clusterRoleBindingObj.Name)

			case "ConfigMap":
				configMapObj := convertToConfigMapObject(runtimeObject)
				if configMapObj.Namespace == "" {
					configMapObj.Namespace = "default"
				}
				err = mc.client.Create(ctx, configMapObj)
				if err != nil {
					mc.logger.Info("Failed to create configmap:", "Error", err)
					return nil, err
				}
				mc.logger.Info("configmap created successfully:", "ConfigMap", configMapObj.Name)

			case "CustomResourceDefinition":
				crdObj := convertToCRDObject(runtimeObject)

				err = mc.client.Create(ctx, crdObj)
				if err != nil {
					mc.logger.Info("Failed to create crd:", "Error", err)
					return nil, err
				}
				mc.logger.Info("crd created successfully:", "CRD", crdObj.Name)

			case "ValidatingWebhookConfiguration":
				webhookObj := convertToValidatingWebhookObject(runtimeObject)

				err = mc.client.Create(ctx, webhookObj)
				if err != nil {
					mc.logger.Info("Failed to create validating webhook:", "Error", err)
					return nil, err
				}
				mc.logger.Info("validating webhook created successfully:", "ValidatingWebhook", webhookObj.Name)

			default:
				mc.logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	manifest := &boundlessv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: boundlessv1alpha1.ManifestSpec{
			Url: url,
		},
	}

	err := mc.client.Create(ctx, manifest)
	if err != nil {
		mc.logger.Info("failed to create manifest crd", "Error", err)
		return nil, err
	}
	mc.logger.Info("manifest created successfully", "ManifestName", "name")

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

func convertToRoleObject(obj runtime.Object) *rbac_v1.Role {
	myobj := obj.(*rbac_v1.Role)
	return myobj
}

func convertToClusterRoleObject(obj runtime.Object) *rbac_v1.ClusterRole {
	myobj := obj.(*rbac_v1.ClusterRole)
	return myobj
}

func convertToRoleBindingObject(obj runtime.Object) *rbac_v1.RoleBinding {
	myobj := obj.(*rbac_v1.RoleBinding)
	return myobj
}

func convertToClusterRoleBindingObject(obj runtime.Object) *rbac_v1.ClusterRoleBinding {
	myobj := obj.(*rbac_v1.ClusterRoleBinding)
	return myobj
}

func convertToSecretObject(obj runtime.Object) *core_v1.Secret {
	myobj := obj.(*core_v1.Secret)
	return myobj
}

func convertToConfigMapObject(obj runtime.Object) *core_v1.ConfigMap {
	myobj := obj.(*core_v1.ConfigMap)
	return myobj
}

func convertToValidatingWebhookObject(obj runtime.Object) *adm_v1.ValidatingWebhookConfiguration {
	myobj := obj.(*adm_v1.ValidatingWebhookConfiguration)
	return myobj
}
