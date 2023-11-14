package controllers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	adm_v1 "k8s.io/api/admissionregistration/v1"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	policy_v1 "k8s.io/api/policy/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

// ManifestReconciler reconciles a Manifest object
type ManifestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	//m      map[string]string
}

var m = make(map[string]string)

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Manifest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ManifestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	logger := log.FromContext(ctx)
	logger.Info("Reconcile request on Manifest instance")

	key := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	logger.Info("Sakshi:: key details", "Namespace", req.Namespace, "Name", req.Name)

	existing := &boundlessv1alpha1.Manifest{}
	err := r.Client.Get(ctx, key, existing)

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("manifest does not exist", "Namespace", req.Namespace, "Name", req.Name)
			// This means that manifest crd deletion workflow has been initiated.
			// Start cleaning up manifest objects

			url, ok := m[req.Name]
			if !ok {
				logger.Error(err, "failed to get url from cache")
				return ctrl.Result{}, err
			}

			logger.Info("url retrived successfully from cache", "URL", url)
			_, err = r.DeleteManifestObjects(url, logger)
			/*if err != nil
			{

			}*/
			return ctrl.Result{}, nil
		} else {
			logger.Error(err, "failed to get manifest object")
			return ctrl.Result{}, err
		}
		//logger.Error(err, "failed to get manifest object")
		//return ctrl.Result{}, err
	}

	logger.Info("url received in manifest object", "URL", existing.Spec.Url)

	m[req.Name] = existing.Spec.Url

	var Client http.Client

	// Run http get request to fetch the contents of the manifest file
	resp, err := Client.Get(existing.Spec.Url)
	if err != nil {
		logger.Error(err, "failed to read response")
		return ctrl.Result{}, err
	}

	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			logger.Error(err, "failed to read http response body")
			return ctrl.Result{}, err
		}

	} else {
		logger.Error(err, "failure in http get request", "ResponseCode", resp.StatusCode)
		return ctrl.Result{}, err
	}
	_, err = r.CreateManifestObjects(bodyBytes, logger)
	/*if err != nil
	{

	}*/

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Manifest{}).
		Complete(r)
}

func (r *ManifestReconciler) CreateManifestObjects(data []byte, logger logr.Logger) ([]*runtime.Object, error) {
	apiextensionsv1.AddToScheme(clientgoscheme.Scheme)
	apiextensionsv1beta1.AddToScheme(clientgoscheme.Scheme)
	adm_v1.AddToScheme(clientgoscheme.Scheme)
	apps_v1.AddToScheme(clientgoscheme.Scheme)
	core_v1.AddToScheme(clientgoscheme.Scheme)
	policy_v1.AddToScheme(clientgoscheme.Scheme)
	rbac_v1.AddToScheme(clientgoscheme.Scheme)

	decoder := clientgoscheme.Codecs.UniversalDeserializer()

	for _, obj := range strings.Split(string(data), "---") {
		if obj != "" {
			runtimeObject, groupVersionKind, err := decoder.Decode([]byte(obj), nil, nil)
			if err != nil {
				logger.Info("Failed to decode yaml:", "Error", err)
				return nil, err
			}

			logger.Info("Decode details", "runtimeObject", runtimeObject, "groupVersionKind", groupVersionKind)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			logger.Info("The object recvd is:", "Kind", groupVersionKind.Kind)

			switch groupVersionKind.Kind {
			case "Namespace":
				// @TODO: create the namespace if it doesn't exist
				namespaceObj := convertToNamespaceObject(runtimeObject)
				err := r.Client.Create(ctx, namespaceObj)
				if err != nil {
					if strings.Contains(err.Error(), "already exists") {
						logger.Info("Namespace already exists:", "Namespace", namespaceObj.Name)
						return nil, nil
					}
					return nil, err
				}
				logger.Info("Namespace created successfully:", "Namespace", namespaceObj.Name)

			case "Service":
				// @TODO: create the service if it doesn't exist
				serviceObj := convertToServiceObject(runtimeObject)
				if serviceObj.Namespace == "" {
					serviceObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, serviceObj)
				if err != nil {
					logger.Info("Failed to create service:", "Error", err)
					return nil, err
				}
				logger.Info("Service created successfully:", "Service", serviceObj.Name)

			case "Deployment":
				// @TODO: create the deployment if it doesn't exist
				deploymentObj := convertToDeploymentObject(runtimeObject)
				if deploymentObj.Namespace == "" {
					deploymentObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, deploymentObj)
				if err != nil {
					logger.Info("Failed to create deployment:", "Error", err)
					return nil, err
				}
				logger.Info("Deployment created successfully:", "Deployment", deploymentObj.Name)

			case "DaemonSet":
				// @TODO: create the daemonSet if it doesn't exist
				daemonsetObj := convertToDaemonsetObject(runtimeObject)
				if daemonsetObj.Namespace == "" {
					daemonsetObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, daemonsetObj)
				if err != nil {
					logger.Info("Failed to create daemonset:", "Error", err)
					return nil, err
				}
				logger.Info("daemonset created successfully:", "Daemonset", daemonsetObj.Name)

			case "PodDisruptionBudget":
				pdbObj := convertToPodDisruptionBudget(runtimeObject)
				if pdbObj.Namespace == "" {
					pdbObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, pdbObj)
				if err != nil {
					logger.Info("Failed to create pod disruption budget:", "Error", err)
					return nil, err
				}
				logger.Info("Pod disruption budget created successfully:", "PodDisruptionBudget", pdbObj.Name)

			case "ServiceAccount":
				serviceAccObj := convertToServiceAccount(runtimeObject)
				if serviceAccObj.Namespace == "" {
					serviceAccObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, serviceAccObj)
				if err != nil {
					logger.Info("Failed to create service account:", "Error", err)
					return nil, err
				}
				logger.Info("service account created successfully:", "ServiceAccount", serviceAccObj.Name)

			case "Role":
				roleObj := convertToRoleObject(runtimeObject)
				if roleObj.Namespace == "" {
					roleObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, roleObj)
				if err != nil {
					logger.Info("Failed to create role:", "Error", err)
					return nil, err
				}
				logger.Info("Role created successfully:", "Role", roleObj.Name)

			case "ClusterRole":
				clusterRoleObj := convertToClusterRoleObject(runtimeObject)
				if clusterRoleObj.Namespace == "" {
					clusterRoleObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, clusterRoleObj)
				if err != nil {
					logger.Info("Failed to create clusterrole:", "Error", err)
					return nil, err
				}
				logger.Info("ClusterRole created successfully:", "Role", clusterRoleObj.Name)

			case "Secret":
				secretObj := convertToSecretObject(runtimeObject)
				if secretObj.Namespace == "" {
					secretObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, secretObj)
				if err != nil {
					logger.Info("Failed to create secret:", "Error", err)
					return nil, err
				}
				logger.Info("secret created successfully:", "Secret", secretObj.Name)

			case "RoleBinding":
				roleBindingObj := convertToRoleBindingObject(runtimeObject)
				if roleBindingObj.Namespace == "" {
					roleBindingObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, roleBindingObj)
				if err != nil {
					logger.Info("Failed to create role binding:", "Error", err)
					return nil, err
				}
				logger.Info("role binding created successfully:", "RoleBinding", roleBindingObj.Name)

			case "ClusterRoleBinding":
				clusterRoleBindingObj := convertToClusterRoleBindingObject(runtimeObject)
				logger.Info("Creating cluster role binding", "Name", clusterRoleBindingObj.Name)

				err = r.Client.Create(ctx, clusterRoleBindingObj)
				if err != nil {
					logger.Info("Failed to create cluster role binding:", "Error", err)
					return nil, err
				}
				logger.Info("cluster role binding created successfully:", "ClusterRoleBinding", clusterRoleBindingObj.Name)

			case "ConfigMap":
				configMapObj := convertToConfigMapObject(runtimeObject)
				if configMapObj.Namespace == "" {
					configMapObj.Namespace = "default"
				}
				err = r.Client.Create(ctx, configMapObj)
				if err != nil {
					logger.Info("Failed to create configmap:", "Error", err)
					return nil, err
				}
				logger.Info("configmap created successfully:", "ConfigMap", configMapObj.Name)

			case "CustomResourceDefinition":
				crdObj := convertToCRDObject(runtimeObject)

				err = r.Client.Create(ctx, crdObj)
				if err != nil {
					logger.Info("Failed to create crd:", "Error", err)
					return nil, err
				}
				logger.Info("crd created successfully:", "CRD", crdObj.Name)

			case "ValidatingWebhookConfiguration":
				webhookObj := convertToValidatingWebhookObject(runtimeObject)

				err = r.Client.Create(ctx, webhookObj)
				if err != nil {
					logger.Info("Failed to create validating webhook:", "Error", err)
					return nil, err
				}
				logger.Info("validating webhook created successfully:", "ValidatingWebhook", webhookObj.Name)

			default:
				logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
			}
		}
	}

	return nil, nil
}

func (r *ManifestReconciler) DeleteManifestObjects(url string, logger logr.Logger) ([]*runtime.Object, error) {
	apiextensionsv1.AddToScheme(clientgoscheme.Scheme)
	apiextensionsv1beta1.AddToScheme(clientgoscheme.Scheme)
	adm_v1.AddToScheme(clientgoscheme.Scheme)
	apps_v1.AddToScheme(clientgoscheme.Scheme)
	core_v1.AddToScheme(clientgoscheme.Scheme)
	policy_v1.AddToScheme(clientgoscheme.Scheme)
	rbac_v1.AddToScheme(clientgoscheme.Scheme)

	var Client http.Client

	// Run http get request to fetch the contents of the manifest file
	resp, err := Client.Get(url)
	if err != nil {
		logger.Error(err, "failed to read http get response")
		return nil, err
	}

	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			logger.Error(err, "failed to read http response body")
			return nil, err
		}

	} else {
		logger.Error(err, "failure in http get request", "ResponseCode", resp.StatusCode)
		return nil, err
	}

	decoder := clientgoscheme.Codecs.UniversalDeserializer()

	for _, obj := range strings.Split(string(bodyBytes), "---") {
		if obj != "" {
			runtimeObject, groupVersionKind, err := decoder.Decode([]byte(obj), nil, nil)
			if err != nil {
				logger.Info("Failed to decode yaml:", "Error", err)
				return nil, err
			}

			logger.Info("Decode details", "runtimeObject", runtimeObject, "groupVersionKind", groupVersionKind)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			logger.Info("The object recvd is:", "Kind", groupVersionKind.Kind)

			switch groupVersionKind.Kind {
			case "Namespace":
				namespaceObj := convertToNamespaceObject(runtimeObject)
				err := r.Client.Delete(ctx, namespaceObj)
				if err != nil {
					logger.Info("Failed to delete Namespace:", "Namespace", namespaceObj.Name)
				} else {
					logger.Info("Namespace deleted successfully:", "Namespace", namespaceObj.Name)
				}

			case "Service":
				serviceObj := convertToServiceObject(runtimeObject)
				if serviceObj.Namespace == "" {
					serviceObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, serviceObj)
				if err != nil {
					logger.Info("Failed to delete service:", "Error", err)
				} else {
					logger.Info("Service deleted successfully:", "Service", serviceObj.Name)
				}

			case "Deployment":
				deploymentObj := convertToDeploymentObject(runtimeObject)
				if deploymentObj.Namespace == "" {
					deploymentObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, deploymentObj)
				if err != nil {
					logger.Info("Failed to delete deployment:", "Error", err)
				} else {
					logger.Info("Deployment deleted successfully:", "Deployment", deploymentObj.Name)
				}

			case "DaemonSet":
				daemonsetObj := convertToDaemonsetObject(runtimeObject)
				if daemonsetObj.Namespace == "" {
					daemonsetObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, daemonsetObj)
				if err != nil {
					logger.Info("Failed to delete daemonset:", "Error", err)
				} else {
					logger.Info("daemonset deleted successfully:", "Daemonset", daemonsetObj.Name)
				}

			case "PodDisruptionBudget":
				pdbObj := convertToPodDisruptionBudget(runtimeObject)
				if pdbObj.Namespace == "" {
					pdbObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, pdbObj)
				if err != nil {
					logger.Info("Failed to delete pod disruption budget:", "Error", err)
				} else {
					logger.Info("Pod disruption budget deleted successfully:", "PodDisruptionBudget", pdbObj.Name)
				}

			case "ServiceAccount":
				serviceAccObj := convertToServiceAccount(runtimeObject)
				if serviceAccObj.Namespace == "" {
					serviceAccObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, serviceAccObj)
				if err != nil {
					logger.Info("Failed to delete service account:", "Error", err)
				} else {
					logger.Info("service account deleted successfully:", "ServiceAccount", serviceAccObj.Name)
				}

			case "Role":
				roleObj := convertToRoleObject(runtimeObject)
				if roleObj.Namespace == "" {
					roleObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, roleObj)
				if err != nil {
					logger.Info("Failed to delete role:", "Error", err)
				} else {
					logger.Info("Role deleted successfully:", "Role", roleObj.Name)
				}

			case "ClusterRole":
				clusterRoleObj := convertToClusterRoleObject(runtimeObject)
				if clusterRoleObj.Namespace == "" {
					clusterRoleObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, clusterRoleObj)
				if err != nil {
					logger.Info("Failed to delete clusterrole:", "Error", err)
				} else {
					logger.Info("ClusterRole deleted successfully:", "Role", clusterRoleObj.Name)
				}

			case "Secret":
				secretObj := convertToSecretObject(runtimeObject)
				if secretObj.Namespace == "" {
					secretObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, secretObj)
				if err != nil {
					logger.Info("Failed to delete secret:", "Error", err)
				} else {
					logger.Info("secret deleted successfully:", "Secret", secretObj.Name)
				}

			case "RoleBinding":
				roleBindingObj := convertToRoleBindingObject(runtimeObject)
				if roleBindingObj.Namespace == "" {
					roleBindingObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, roleBindingObj)
				if err != nil {
					logger.Info("Failed to delete role binding:", "Error", err)
				} else {
					logger.Info("role binding deleted successfully:", "RoleBinding", roleBindingObj.Name)
				}

			case "ClusterRoleBinding":
				clusterRoleBindingObj := convertToClusterRoleBindingObject(runtimeObject)

				err = r.Client.Delete(ctx, clusterRoleBindingObj)
				if err != nil {
					logger.Info("Failed to delete cluster role binding:", "Error", err)
				} else {
					logger.Info("cluster role binding deleted successfully:", "ClusterRoleBinding", clusterRoleBindingObj.Name)
				}

			case "ConfigMap":
				configMapObj := convertToConfigMapObject(runtimeObject)
				if configMapObj.Namespace == "" {
					configMapObj.Namespace = "default"
				}
				err = r.Client.Delete(ctx, configMapObj)
				if err != nil {
					logger.Info("Failed to delete configmap:", "Error", err)
				} else {
					logger.Info("configmap deleted successfully:", "ConfigMap", configMapObj.Name)
				}

			case "CustomResourceDefinition":
				crdObj := convertToCRDObject(runtimeObject)

				err = r.Client.Delete(ctx, crdObj)
				if err != nil {
					logger.Info("Failed to delete crd:", "Error", err)
				} else {
					logger.Info("crd deleted successfully:", "CRD", crdObj.Name)
				}

			case "ValidatingWebhookConfiguration":
				webhookObj := convertToValidatingWebhookObject(runtimeObject)

				err = r.Client.Delete(ctx, webhookObj)
				if err != nil {
					logger.Info("Failed to delete validating webhook:", "Error", err)
				} else {
					logger.Info("validating webhook deleted successfully:", "ValidatingWebhook", webhookObj.Name)
				}

			default:
				logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
			}
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
