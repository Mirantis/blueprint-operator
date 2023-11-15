package controllers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
}

type ManifestObjects struct {
	ApiVersion string
	Kind       string
	Name       string
	Group      string
	Namespace  string
}

// The checkSum map stores the checksum for each manifest.
// Storing this value is crucial as this will help the manifest controller
// differentiate between create and update requests.
var checkSum = make(map[string]string)

// The objs map stores the list of objects in a manifest.
// This list will be used by the manifest controller to track all the
// objects in case of delete and update.
var objs = make(map[string][]ManifestObjects)

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

	existing := &boundlessv1alpha1.Manifest{}
	err := r.Client.Get(ctx, key, existing)

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("manifest does not exist", "Namespace", req.Namespace, "Name", req.Name)
			return ctrl.Result{}, nil
		} else {
			logger.Error(err, "failed to get manifest object")
			return ctrl.Result{}, err
		}
	}

	addonFinalizerName := "manifest/finalizer"

	if existing.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(existing, addonFinalizerName) {
			controllerutil.AddFinalizer(existing, addonFinalizerName)
			if err := r.Update(ctx, existing); err != nil {
				logger.Info("failed to update manifest object with finalizer", "Name", req.Name, "Finalizer", addonFinalizerName)
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(existing, addonFinalizerName) {
			// The finalizer is present, so lets delete the objects for this manifest

			if err := r.DeleteManifestObjects(req, ctx); err != nil {
				logger.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}

			// Delete entry from map
			delete(objs, req.Name)

			// Remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(existing, addonFinalizerName)
			if err := r.Update(ctx, existing); err != nil {
				logger.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Validate checksum entry in the checkSum map
	sum, ok := checkSum[req.Name]
	if !ok {
		// Entry not present, add it.
		// This will happen in case of the create request.
		checkSum[req.Name] = existing.Spec.Checksum
	} else {
		// Present. Compare it with the new request.
		if sum == existing.Spec.Checksum {
			// Do nothing
			logger.Info("checksum is same, no update needed", "Cache", sum, "Object", existing.Spec.Checksum)
			return ctrl.Result{}, nil
		} else {
			logger.Info("checksum is not same, update needed", "Cache", sum, "Object", existing.Spec.Checksum)
			// @TODO : Add code for update
			return ctrl.Result{}, nil
		}
	}

	// We will reach here only in case of create request.
	// Run http get request to fetch the contents of the manifest file.
	var Client http.Client
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
	err = r.CreateManifestObjects(req, bodyBytes, logger)
	if err != nil {
		logger.Error(err, "failed to create manifest objects", "ResponseCode", resp.StatusCode)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Manifest{}).
		Complete(r)
}

func (r *ManifestReconciler) CreateManifestObjects(req ctrl.Request, data []byte, logger logr.Logger) error {
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
				return err
			}

			//logger.Info("Decode details", "runtimeObject", runtimeObject, "groupVersionKind", groupVersionKind)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			logger.Info("The object recvd is:", "Kind", groupVersionKind.Kind)

			switch groupVersionKind.Kind {
			case "Namespace":
				err := r.addNamespaceObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "Service":
				err := r.addServiceObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "Deployment":
				err := r.addDeploymentObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "DaemonSet":
				err := r.addDaemonsetObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "PodDisruptionBudget":
				err := r.addPodDisruptionBudget(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "ServiceAccount":
				err := r.addServiceAccount(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "Role":
				err := r.addRoleObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "ClusterRole":
				err := r.addClusterRoleObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "Secret":
				err := r.addSecretObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "RoleBinding":
				err := r.addRoleBindingObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "ClusterRoleBinding":
				err := r.addClusterRoleBindingObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "ConfigMap":
				err := r.addConfigMapObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "CustomResourceDefinition":
				err := r.addCRDObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			case "ValidatingWebhookConfiguration":
				err := r.addValidatingWebhookObject(runtimeObject, groupVersionKind, req, ctx)
				if err != nil {
					return err
				}

			default:
				logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
			}
		}
	}

	return nil
}

func (r *ManifestReconciler) addNamespaceObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)

	myobj := obj.(*core_v1.Namespace)
	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("namespace already exists:", "Namespace", myobj.Name)
			return nil
		}
		logger.Info("failed to create namespace:", "Error", err)
		return err
	}

	logger.Info("namespace created successfully:", "Namespace", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  "",
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("namespace object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addServiceObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)

	myobj := obj.(*core_v1.Service)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("service already exists:", "Service", myobj.Name)
			return nil
		}
		logger.Info("failed to create service:", "Error", err)
		return err
	}

	logger.Info("service created successfully:", "Service", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("service object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addDeploymentObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*apps_v1.Deployment)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("deployment already exists:", "Deployment", myobj.Name)
			return nil
		}
		logger.Info("failed to create deployment:", "Error", err)
		return err
	}

	logger.Info("deployment created successfully:", "Deployment", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("deployment object added successfully to the list")

	return nil

}

func (r *ManifestReconciler) addPodDisruptionBudget(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*policy_v1.PodDisruptionBudget)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("pod disruption budget already exists:", "PodDisruption", myobj.Name)
			return nil
		}
		logger.Info("failed to create pod disruption budget:", "Error", err)
		return err
	}

	logger.Info("pod disruption budget created successfully:", "PodDisruption", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("pod disruption object added successfully to the list")

	return nil

}

func (r *ManifestReconciler) addServiceAccount(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*core_v1.ServiceAccount)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("service account already exists:", "ServiceAcoount", myobj.Name)
			return nil
		}
		logger.Info("failed to create service account:", "Error", err)
		return err
	}

	logger.Info("service account created successfully:", "ServiceAccount", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("service account object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addCRDObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*apiextensionsv1.CustomResourceDefinition)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("crd already exists:", "CRD", myobj.Name)
			return nil
		}
		logger.Info("failed to create crd:", "Error", err)
		return err
	}

	logger.Info("crd created successfully:", "CRD", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  "",
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("crd object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addDaemonsetObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*apps_v1.DaemonSet)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("daemonset already exists:", "Daemonset", myobj.Name)
			return nil
		}
		logger.Info("failed to create daemonset:", "Error", err)
		return err
	}

	logger.Info("daemonset created successfully:", "Daemonset", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("daemonset object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addRoleObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*rbac_v1.Role)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("role already exists:", "Role", myobj.Name)
			return nil
		}
		logger.Info("failed to create role:", "Error", err)
		return err
	}

	logger.Info("role created successfully:", "Role", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("role object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addClusterRoleObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*rbac_v1.ClusterRole)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("clusterrole already exists:", "Clusterrole", myobj.Name)
			return nil
		}
		logger.Info("failed to create clusterrole:", "Error", err)
		return err
	}

	logger.Info("clusterrole created successfully:", "Clusterrole", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("clusterrole object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addRoleBindingObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*rbac_v1.RoleBinding)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("rolebinding already exists:", "Rolebinding", myobj.Name)
			return nil
		}
		logger.Info("failed to create rolebinding:", "Error", err)
		return err
	}

	logger.Info("rolebinding created successfully:", "Rolebinding", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("rolebinding object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addClusterRoleBindingObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*rbac_v1.ClusterRoleBinding)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("clusterrolebinding already exists:", "ClusterRoleBinding", myobj.Name)
			return nil
		}
		logger.Info("failed to create cluster role binding:", "Error", err)
		return err
	}

	logger.Info("cluster role binding created successfully:", "ClusterRoleBinding", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  "",
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("cluster role binding object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addSecretObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*core_v1.Secret)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("secret already exists:", "Secret", myobj.Name)
			return nil
		}
		logger.Info("failed to create secret:", "Error", err)
		return err
	}

	logger.Info("secret created successfully:", "Secret", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("secret object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addConfigMapObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*core_v1.ConfigMap)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("configmap already exists:", "Configmap", myobj.Name)
			return nil
		}
		logger.Info("failed to create configmap:", "Error", err)
		return err
	}

	logger.Info("configmap created successfully:", "Configmap", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  myobj.Namespace,
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("configmap object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) addValidatingWebhookObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*adm_v1.ValidatingWebhookConfiguration)

	err := r.Client.Create(ctx, myobj)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Info("validating webhook already exists:", "ValidatingWebhook", myobj.Name)
			return nil
		}
		logger.Info("failed to create validating webhook:", "Error", err)
		return err
	}

	logger.Info("validating webhook created successfully:", "ValidatingWebhook", myobj.Name)

	// Add this object to the list
	newObj := ManifestObjects{
		ApiVersion: groupVersionKind.Version,
		Kind:       groupVersionKind.Kind,
		Name:       myobj.Name,
		Group:      groupVersionKind.Group,
		Namespace:  "",
	}
	objs[req.Name] = append(objs[req.Name], newObj)
	logger.Info("validating webhook object added successfully to the list")

	return nil
}

func (r *ManifestReconciler) DeleteManifestObjects(req ctrl.Request, ctx context.Context) error {
	logger := log.FromContext(ctx)
	// Fetch all the objects stored in the manifest cache and delete them
	for _, val := range objs[req.Name] {

		switch val.Kind {
		case "Namespace":
			err := r.deleteNamespaceObject(val, ctx)
			if err != nil {
				return err
			}
		case "Service":
			err := r.deleteServiceObject(val, ctx)
			if err != nil {
				return err
			}

		case "Deployment":
			err := r.deleteDeploymentObject(val, ctx)
			if err != nil {
				return err
			}

		case "DaemonSet":
			err := r.deleteDaemonsetObject(val, ctx)
			if err != nil {
				return err
			}

		case "PodDisruptionBudget":
			err := r.deletePDBObject(val, ctx)
			if err != nil {
				return err
			}

		case "ServiceAccount":
			err := r.deleteServiceAccountObject(val, ctx)
			if err != nil {
				return err
			}

		case "Role":
			err := r.deleteRoleObject(val, ctx)
			if err != nil {
				return err
			}

		case "ClusterRole":
			err := r.deleteClusterRoleObject(val, ctx)
			if err != nil {
				return err
			}

		case "Secret":
			err := r.deleteSecretObject(val, ctx)
			if err != nil {
				return err
			}

		case "RoleBinding":
			err := r.deleteRoleBindingObject(val, ctx)
			if err != nil {
				return err
			}

		case "ClusterRoleBinding":
			err := r.deleteClusterRoleBindingObject(val, ctx)
			if err != nil {
				return err
			}

		case "ConfigMap":
			err := r.deleteConfigmapObject(val, ctx)
			if err != nil {
				return err
			}

		case "CustomResourceDefinition":
			err := r.deleteCRDObject(val, ctx)
			if err != nil {
				return err
			}

		case "ValidatingWebhookConfiguration":
			err := r.deleteValidatingWebhookObject(val, ctx)
			if err != nil {
				return err
			}

		default:
			logger.Info("Object kind not supported", "Kind", val.Kind)
		}

	}
	return nil
}

func (r *ManifestReconciler) deleteNamespaceObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	namespace := &core_v1.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, namespace)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("namespace does not exist", "Namespace", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get namespace object")
			return err
		}
	}

	logger.Info("namespace object retrived successfully:", "Namespace", namespace)

	err = r.Client.Delete(ctx, namespace)
	if err != nil {
		logger.Info("failed to delete namespace:", "Error", err)
		return err
	} else {
		logger.Info("namespace deleted successfully:", "Namespace", namespace.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteServiceObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	service := &core_v1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, service)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("service does not exist", "Service", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get service object")
			return err
		}
	}

	logger.Info("service object retrived successfully:", "Service", service)

	err = r.Client.Delete(ctx, service)
	if err != nil {
		logger.Info("failed to delete service:", "Error", err)
		return err
	} else {
		logger.Info("service deleted successfully:", "Service", service.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteServiceAccountObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	serviceAccount := &core_v1.ServiceAccount{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, serviceAccount)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("service account does not exist", "Service", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get service account object")
			return err
		}
	}

	err = r.Client.Delete(ctx, serviceAccount)
	if err != nil {
		logger.Info("failed to delete service account:", "Error", err)
		return err
	} else {
		logger.Info("service account deleted successfully:", "Service", serviceAccount.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteCRDObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, crd)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("crd does not exist", "Service", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get crd object")
			return err
		}
	}

	err = r.Client.Delete(ctx, crd)
	if err != nil {
		logger.Info("failed to delete crd:", "Error", err)
		return err
	} else {
		logger.Info("crd deleted successfully:", "CRD", crd.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteDeploymentObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	deployment := &apps_v1.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, deployment)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("deployment does not exist", "Deployment", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get deployment object")
			return err
		}
	}

	err = r.Client.Delete(ctx, deployment)
	if err != nil {
		logger.Info("failed to delete deployment:", "Error", err)
		return err
	} else {
		logger.Info("deployment deleted successfully:", "Deployment", deployment.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteDaemonsetObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	daemonset := &apps_v1.DaemonSet{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, daemonset)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("daemonset does not exist", "Daemonset", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get daemonset object")
			return err
		}
	}

	err = r.Client.Delete(ctx, daemonset)
	if err != nil {
		logger.Info("failed to delete daemonset:", "Error", err)
		return err
	} else {
		logger.Info("daemonset deleted successfully:", "Daemonset", daemonset.Name)
	}

	return nil
}

func (r *ManifestReconciler) deletePDBObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	pdb := &policy_v1.PodDisruptionBudget{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, pdb)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("policy discruption budget does not exist", "policyDiscruptionBudget", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get policy discruption budget object")
			return err
		}
	}

	err = r.Client.Delete(ctx, pdb)
	if err != nil {
		logger.Info("failed to delete policy discruption budget:", "Error", err)
		return err
	} else {
		logger.Info("policy discruption budget deleted successfully:", "PolicyDiscruptionBudget", pdb.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteRoleObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	role := &rbac_v1.Role{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, role)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("role does not exist", "Role", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get role object")
			return err
		}
	}

	err = r.Client.Delete(ctx, role)
	if err != nil {
		logger.Info("failed to delete role:", "Error", err)
		return err
	} else {
		logger.Info("role deleted successfully:", "Role", role.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteClusterRoleObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	clusterRole := &rbac_v1.ClusterRole{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, clusterRole)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("clusterRole does not exist", "ClusterRole", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get clusterRole object")
			return err
		}
	}

	err = r.Client.Delete(ctx, clusterRole)
	if err != nil {
		logger.Info("failed to delete clusterRole:", "Error", err)
		return err
	} else {
		logger.Info("clusterRole deleted successfully:", "ClusterRole", clusterRole.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteSecretObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	secret := &core_v1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, secret)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("secret does not exist", "Secret", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get secret object")
			return err
		}
	}

	err = r.Client.Delete(ctx, secret)
	if err != nil {
		logger.Info("failed to delete secret:", "Error", err)
		return err
	} else {
		logger.Info("secret deleted successfully:", "Secret", secret.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteRoleBindingObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	roleBinding := &rbac_v1.RoleBinding{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, roleBinding)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("roleBinding does not exist", "RoleBinding", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get roleBinding object")
			return err
		}
	}

	err = r.Client.Delete(ctx, roleBinding)
	if err != nil {
		logger.Info("failed to delete roleBinding:", "Error", err)
		return err
	} else {
		logger.Info("roleBinding deleted successfully:", "RoleBinding", roleBinding.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteClusterRoleBindingObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	clusterRoleBinding := &rbac_v1.ClusterRoleBinding{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, clusterRoleBinding)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("clusterRoleBinding does not exist", "ClusterRoleBinding", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get clusterRoleBinding object")
			return err
		}
	}

	err = r.Client.Delete(ctx, clusterRoleBinding)
	if err != nil {
		logger.Info("failed to delete clusterRoleBinding:", "Error", err)
		return err
	} else {
		logger.Info("clusterRoleBinding deleted successfully:", "ClusterRoleBinding", clusterRoleBinding.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteConfigmapObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	configMap := &core_v1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, configMap)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("configMap does not exist", "ConfigMap", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get configMap object")
			return err
		}
	}

	err = r.Client.Delete(ctx, configMap)
	if err != nil {
		logger.Info("failed to delete configMap:", "Error", err)
		return err
	} else {
		logger.Info("configMap deleted successfully:", "ConfigMap", configMap.Name)
	}

	return nil
}

func (r *ManifestReconciler) deleteValidatingWebhookObject(val ManifestObjects, ctx context.Context) error {
	logger := log.FromContext(ctx)

	webhook := &adm_v1.ValidatingWebhookConfiguration{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: val.Namespace,
		Name:      val.Name,
	}, webhook)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("webhook does not exist", "Webhook", val.Name)
			return nil
		} else {
			logger.Error(err, "failed to get webhook object")
			return err
		}
	}

	err = r.Client.Delete(ctx, webhook)
	if err != nil {
		logger.Info("failed to delete webhook:", "Error", err)
		return err
	} else {
		logger.Info("webhook deleted successfully:", "Webhook", webhook.Name)
	}

	return nil
}
