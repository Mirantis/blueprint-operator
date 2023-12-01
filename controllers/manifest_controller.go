package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	adm_v1 "k8s.io/api/admissionregistration/v1"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	policy_v1 "k8s.io/api/policy/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"
	"github.com/mirantis/boundless-operator/pkg/event"
)

const (
	actionUpdate = "update"
	actionCreate = "create"
	actionDelete = "delete"
)

// ManifestReconciler reconciles a Manifest object
type ManifestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

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
				r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest object with finalizer %s/%s", existing.Namespace, existing.Name)
				r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to update manifest object with finalizer", fmt.Sprintf("failed to update manifest object with finalizer : %s", err))
				return ctrl.Result{Requeue: true}, err
			}
			logger.Info("finalizer added successfully", "Name", req.Name, "Finalizer", addonFinalizerName)
			return ctrl.Result{}, err
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(existing, addonFinalizerName) {
			// The finalizer is present, so lets delete the objects for this manifest
			if err := r.DeleteManifestObjects(existing.Spec.Objects, ctx); err != nil {
				logger.Error(err, "failed to delete manifest objects")
				r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to delete manifest objects %s/%s", existing.Namespace, existing.Name)
				r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to delete manifest objects", fmt.Sprintf("failed to delete manifest objects : %s", err))
				return ctrl.Result{Requeue: true}, err
			}

			// Remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(existing, addonFinalizerName)
			if err := r.Update(ctx, existing); err != nil {
				logger.Error(err, "failed to remove finalizer")
				r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to remove finalizer %s/%s", existing.Namespace, existing.Name)
				r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to remove finalizer", fmt.Sprintf("failed to remove finalizer : %s", err))
				return ctrl.Result{Requeue: true}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if existing.Spec.Checksum == existing.Spec.NewChecksum && existing.Status.Type == boundlessv1alpha1.TypeComponentAvailable {
		logger.Info("checksum is same, no update needed", "Checksum", existing.Spec.Checksum, "NewChecksum", existing.Spec.NewChecksum)
		return ctrl.Result{}, nil
	}

	if (existing.Spec.Checksum != existing.Spec.NewChecksum) && (existing.Spec.NewChecksum != "") {
		// Update is required
		logger.Info("checksum differs, update needed", "Checksum", existing.Spec.Checksum, "NewChecksum", existing.Spec.NewChecksum)
		// First, update the checksum to avoid any reconciliation
		// Update the CRD
		updatedCRD := boundlessv1alpha1.Manifest{
			ObjectMeta: metav1.ObjectMeta{
				Name:            existing.Name,
				Namespace:       existing.Namespace,
				ResourceVersion: existing.ResourceVersion,
			},
			Spec: boundlessv1alpha1.ManifestSpec{
				Url:         existing.Spec.Url,
				Checksum:    existing.Spec.NewChecksum,
				NewChecksum: existing.Spec.NewChecksum,
			},
		}

		if err := r.Update(ctx, &updatedCRD); err != nil {
			logger.Error(err, "failed to update manifest crd while update operation")
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest crd while update operation %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to update manifest crd while update operation", fmt.Sprintf("failed to update manifest crd while update operation : %s", err))
			return ctrl.Result{}, err
		}

		// TODO: https://github.com/Mirantis/boundless-operator/pull/17#pullrequestreview-1754136032
		if err := r.UpdateManifestObjects(req, ctx, existing); err != nil {
			logger.Error(err, "failed to update manifest")
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to update manifest", fmt.Sprintf("failed to update manifest : %s", err))
			return ctrl.Result{}, err
		}

	}

	if existing.Spec.NewChecksum == "" {
		// We will reach here only in case of create request.
		// First, update the checksum in CRD to avoid any reconciliations.
		// Update the CRD
		updatedCRD := boundlessv1alpha1.Manifest{
			ObjectMeta: metav1.ObjectMeta{
				Name:            existing.Name,
				Namespace:       existing.Namespace,
				ResourceVersion: existing.ResourceVersion,
			},
			Spec: boundlessv1alpha1.ManifestSpec{
				Url:         existing.Spec.Url,
				Checksum:    existing.Spec.Checksum,
				NewChecksum: existing.Spec.Checksum,
			},
		}

		if err := r.Update(ctx, &updatedCRD); err != nil {
			logger.Error(err, "failed to update manifest crd while create operation")
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest crd while create operation %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to update manifest crd while create operation", fmt.Sprintf("failed to update manifest crd while create operation : %s", err))
			return ctrl.Result{}, err
		}

		// Run http get request to fetch the contents of the manifest file.
		bodyBytes, err := r.ReadManifest(req, existing.Spec.Url, logger)
		if err != nil {
			logger.Error(err, "failed to fetch manifest file content for url: %s", "Manifest Url", existing.Spec.Url)
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to fetch manifest file content for url %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to fetch manifest file content for url", fmt.Sprintf("failed to fetch manifest file content for url : %s", err))
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}

		logger.Info("received new crd request. Creating manifest objects..")
		err = r.CreateManifestObjects(req, bodyBytes, logger, ctx, existing)
		if err != nil {
			logger.Error(err, "failed to create objects for the manifest", "Name", req.Name)
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to create objects for the manifest %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to create objects for the manifest", fmt.Sprintf("failed to create objects for the manifest : %s", err))
			return ctrl.Result{}, err
		}

	}

	r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Manifest %s/%s", existing.Namespace, existing.Name)
	err = r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentAvailable, "Manifest Created")
	if err != nil {
		logger.Error(err, "Failed to update status after manifest creation")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Manifest{}).
		Complete(r)
}

func (r *ManifestReconciler) CreateManifestObjects(req ctrl.Request, data []byte, logger logr.Logger, ctx context.Context, existing *boundlessv1alpha1.Manifest) error {
	apiextensionsv1.AddToScheme(clientgoscheme.Scheme)
	apiextensionsv1beta1.AddToScheme(clientgoscheme.Scheme)
	adm_v1.AddToScheme(clientgoscheme.Scheme)
	apps_v1.AddToScheme(clientgoscheme.Scheme)
	core_v1.AddToScheme(clientgoscheme.Scheme)
	policy_v1.AddToScheme(clientgoscheme.Scheme)
	rbac_v1.AddToScheme(clientgoscheme.Scheme)

	decoder := clientgoscheme.Codecs.UniversalDeserializer()

	manifestObjs := []boundlessv1alpha1.ManifestObject{}
	val := boundlessv1alpha1.ManifestObject{}

	for _, obj := range strings.Split(string(data), "---") {
		if obj != "" {
			runtimeObject, groupVersionKind, err := decoder.Decode([]byte(obj), nil, nil)
			if err != nil {
				logger.Info("Failed to decode yaml:", "Error", err)
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			switch groupVersionKind.Kind {
			case "Namespace":
				err := r.handleNamespaceObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "Service":
				err := r.handleServiceObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "Deployment":
				err := r.handleDeploymentObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "DaemonSet":
				err := r.handleDaemonsetObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "PodDisruptionBudget":
				err := r.handlePodDisruptionBudget(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "ServiceAccount":
				err := r.handleServiceAccount(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "Role":
				err := r.handleRoleObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "ClusterRole":
				err := r.handleClusterRoleObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "Secret":
				err := r.handleSecretObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "RoleBinding":
				err := r.handleRoleBindingObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "ClusterRoleBinding":
				err := r.handleClusterRoleBindingObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "ConfigMap":
				err := r.handleConfigMapObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "CustomResourceDefinition":
				err := r.handleCRDObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			case "ValidatingWebhookConfiguration":
				err := r.handleValidatingWebhookObject(runtimeObject, groupVersionKind, req, ctx, &manifestObjs, actionCreate, val)
				if err != nil {
					return err
				}

			default:
				logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
			}
		}
	}

	// TODO: https://github.com/Mirantis/boundless-operator/pull/17#discussion_r1408570381
	// Update the CRD
	key := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	crd := &boundlessv1alpha1.Manifest{}
	err := r.Client.Get(ctx, key, crd)
	if err != nil {
		logger.Error(err, "failed to get manifest object")
		return err
	}
	// Update the CRD
	updatedCRD := boundlessv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            crd.Name,
			Namespace:       crd.Namespace,
			ResourceVersion: crd.ResourceVersion,
		},
		Spec: boundlessv1alpha1.ManifestSpec{
			Url:         crd.Spec.Url,
			Checksum:    crd.Spec.Checksum,
			NewChecksum: crd.Spec.NewChecksum,
			Objects:     manifestObjs,
		},
	}

	if err := r.Update(ctx, &updatedCRD); err != nil {
		logger.Error(err, "failed to update manifest crd with objectList during create")
		return err
	}

	return nil
}

func (r *ManifestReconciler) handleNamespaceObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteNamespaceObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*core_v1.Namespace)

		namespace := &core_v1.Namespace{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, namespace)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("namespace does not exist, creating..", "Namespace", myobj.Name)
				// Create the object
				err := r.addNamespaceObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get namespace object")
				return err
			}
		}

		// Update the object in this case
		myobj.ObjectMeta.ResourceVersion = namespace.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update namespace:", "Error", err)
			return err
		} else {
			logger.Info("namespace updated successfully:", "Namespace", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleServiceObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteServiceObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists

		myobj := runtimeObject.(*core_v1.Service)

		service := &core_v1.Service{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, service)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("service does not exist, creating..", "Service", myobj.Name)
				// Create the object
				err := r.addServiceObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get service object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = service.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update service:", "Error", err)
			return err
		} else {
			logger.Info("service updated successfully:", "Service", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleDeploymentObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteDeploymentObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*apps_v1.Deployment)

		deployment := &apps_v1.Deployment{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, deployment)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("deployment does not exist, creating..", "Deployment", myobj.Name)
				err := r.addDeploymentObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil

			} else {
				logger.Error(err, "failed to get deployment object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = deployment.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update deployment:", "Error", err)
			return err
		} else {
			logger.Info("deployment updated successfully:", "Deployment", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleDaemonsetObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteDaemonsetObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*apps_v1.DaemonSet)

		daemonset := &apps_v1.DaemonSet{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, daemonset)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("daemonset does not exist, creating..", "Daemonset", myobj.Name)
				err := r.addDaemonsetObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil

			} else {
				logger.Error(err, "failed to get daemonset object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = daemonset.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update daemonset:", "Error", err)
			return err
		} else {
			logger.Info("daemonset updated successfully:", "Daemonset", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handlePodDisruptionBudget(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deletePDBObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*policy_v1.PodDisruptionBudget)

		pdb := &policy_v1.PodDisruptionBudget{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, pdb)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("policy discruption budget does not exist, creating..", "policyDiscruptionBudget", myobj.Name)
				err := r.addPodDisruptionBudget(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil

			} else {
				logger.Error(err, "failed to get policy discruption budget object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = pdb.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update policy discruption budget:", "Error", err)
			return err
		} else {
			logger.Info("policy discruption budget updated successfully:", "PolicyDiscruptionBudget", pdb.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleServiceAccount(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteServiceAccountObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*core_v1.ServiceAccount)

		serviceAccount := &core_v1.ServiceAccount{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, serviceAccount)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("service account does not exist, creating..", "Service", myobj.Name)
				err := r.addServiceAccount(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get service account object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = serviceAccount.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update service account:", "Error", err)
			return err
		} else {
			logger.Info("service account updated successfully:", "Service", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleRoleObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteRoleObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*rbac_v1.Role)

		role := &rbac_v1.Role{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, role)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("role does not exist, creating..", "Role", myobj.Name)
				err := r.addRoleObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get role object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = role.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update role:", "Error", err)
			return err
		} else {
			logger.Info("role updated successfully:", "Role", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleClusterRoleObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteClusterRoleObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*rbac_v1.ClusterRole)

		clusterRole := &rbac_v1.ClusterRole{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, clusterRole)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("clusterRole does not exist, creating..", "ClusterRole", myobj.Name)
				err := r.addClusterRoleObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get clusterRole object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = clusterRole.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update clusterRole:", "Error", err)
			return err
		} else {
			logger.Info("clusterRole updated successfully:", "ClusterRole", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleSecretObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteSecretObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*core_v1.Secret)

		secret := &core_v1.Secret{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, secret)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("secret does not exist, creating..", "Secret", myobj.Name)
				err := r.addSecretObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get secret object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = secret.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update secret:", "Error", err)
			return err
		} else {
			logger.Info("secret updated successfully:", "Secret", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleRoleBindingObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteRoleBindingObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*rbac_v1.RoleBinding)

		roleBinding := &rbac_v1.RoleBinding{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, roleBinding)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("roleBinding does not exist. creating..", "RoleBinding", myobj.Name)
				err := r.addRoleBindingObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get roleBinding object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = roleBinding.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update roleBinding:", "Error", err)
			return err
		} else {
			logger.Info("roleBinding updated successfully:", "RoleBinding", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleClusterRoleBindingObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteClusterRoleBindingObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*rbac_v1.ClusterRoleBinding)

		clusterRoleBinding := &rbac_v1.ClusterRoleBinding{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, clusterRoleBinding)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("clusterRoleBinding does not exist, creating..", "ClusterRoleBinding", myobj.Name)
				err := r.addClusterRoleBindingObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get clusterRoleBinding object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = clusterRoleBinding.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update clusterRoleBinding:", "Error", err)
			return err
		} else {
			logger.Info("clusterRoleBinding updated successfully:", "ClusterRoleBinding", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleConfigMapObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteConfigmapObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*core_v1.ConfigMap)

		configMap := &core_v1.ConfigMap{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, configMap)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("configMap does not exist, creating..", "ConfigMap", myobj.Name)
				err := r.addConfigMapObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get configMap object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = configMap.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update configMap:", "Error", err)
			return err
		} else {
			logger.Info("configMap updated successfully:", "ConfigMap", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleCRDObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteCRDObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*apiextensionsv1.CustomResourceDefinition)

		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, crd)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("crd does not exist, creating..", "CRD", myobj.Name)
				err := r.addCRDObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get crd object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = crd.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update crd:", "Error", err)
			return err
		} else {
			logger.Info("crd updated successfully:", "CRD", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) handleValidatingWebhookObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	if action == actionDelete {
		err := r.deleteValidatingWebhookObject(val, ctx)
		if err != nil {
			return err
		}

	} else {
		// action = update/create
		// Check if this object exists
		myobj := runtimeObject.(*adm_v1.ValidatingWebhookConfiguration)

		webhook := &adm_v1.ValidatingWebhookConfiguration{}
		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: myobj.Namespace,
			Name:      myobj.Name,
		}, webhook)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Info("webhook does not exist, creating..", "Webhook", myobj.Name)
				err := r.addValidatingWebhookObject(runtimeObject, groupVersionKind, req, ctx, manifestObjs)
				if err != nil {
					return err
				}
				return nil
			} else {
				logger.Error(err, "failed to get webhook object")
				return err
			}
		}

		myobj.ObjectMeta.ResourceVersion = webhook.ResourceVersion
		err = r.Client.Update(ctx, myobj)
		if err != nil {
			logger.Info("failed to update webhook:", "Error", err)
			return err
		} else {
			logger.Info("webhook updated successfully:", "Webhook", myobj.Name)
			// Add this object to the list
			r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
			return nil
		}

	}
	return nil
}

func (r *ManifestReconciler) addNamespaceObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, "", req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addServiceObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	myobj := obj.(*core_v1.Service)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addDeploymentObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*apps_v1.Deployment)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil

}

func (r *ManifestReconciler) addPodDisruptionBudget(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*policy_v1.PodDisruptionBudget)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil

}

func (r *ManifestReconciler) addServiceAccount(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*core_v1.ServiceAccount)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addCRDObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, "", req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addDaemonsetObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*apps_v1.DaemonSet)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addRoleObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*rbac_v1.Role)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addClusterRoleObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*rbac_v1.ClusterRole)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addRoleBindingObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*rbac_v1.RoleBinding)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addClusterRoleBindingObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, "", req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addSecretObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*core_v1.Secret)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addConfigMapObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)
	myobj := obj.(*core_v1.ConfigMap)

	if myobj.Namespace == "" {
		myobj.Namespace = "default"
	}

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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, myobj.Namespace, req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addValidatingWebhookObject(obj runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject) error {
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
	r.addObjectToList(groupVersionKind.Kind, myobj.Name, "", req, manifestObjs)
	return nil
}

func (r *ManifestReconciler) addObjectToList(kind string, name string, namespace string, req ctrl.Request, manifestObjs *[]boundlessv1alpha1.ManifestObject) {

	// Add this object to the list
	// @TODO: Check if we can use dynamic clients to create the objects
	// https://mirantis.jira.com/browse/BOP-102
	updatedObject := boundlessv1alpha1.ManifestObject{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}

	*manifestObjs = append(*manifestObjs, updatedObject)

}

func (r *ManifestReconciler) DeleteManifestObjects(objectList []boundlessv1alpha1.ManifestObject, ctx context.Context) error {
	logger := log.FromContext(ctx)
	var runtimeObject runtime.Object
	var req ctrl.Request

	//handleNamespaceObject(runtimeObject runtime.Object, groupVersionKind *schema.GroupVersionKind, req ctrl.Request, ctx context.Context, manifestObjs *[]boundlessv1alpha1.ManifestObject, action string, val boundlessv1alpha1.ManifestObject) error {

	// Fetch all the objects stored in the manifest object list and delete them
	if len(objectList) > 0 {
		for _, val := range objectList {

			switch val.Kind {
			case "Namespace":
				err := r.handleNamespaceObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}
			case "Service":
				err := r.handleServiceObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "Deployment":
				err := r.handleDeploymentObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "DaemonSet":
				err := r.handleDaemonsetObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "PodDisruptionBudget":
				err := r.handlePodDisruptionBudget(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "ServiceAccount":
				err := r.handleServiceAccount(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "Role":
				err := r.handleRoleObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "ClusterRole":
				err := r.handleClusterRoleObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "Secret":
				err := r.handleSecretObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "RoleBinding":
				err := r.handleRoleBindingObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "ClusterRoleBinding":
				err := r.handleClusterRoleBindingObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "ConfigMap":
				err := r.handleConfigMapObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "CustomResourceDefinition":
				err := r.handleCRDObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			case "ValidatingWebhookConfiguration":
				err := r.handleValidatingWebhookObject(runtimeObject, nil, req, ctx, nil, actionDelete, val)
				if err != nil {
					return err
				}

			default:
				logger.Info("Object kind not supported", "Kind", val.Kind)
			}

		}
	}
	return nil
}

func (r *ManifestReconciler) deleteNamespaceObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteServiceObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteServiceAccountObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteCRDObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteDeploymentObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteDaemonsetObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deletePDBObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteRoleObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteClusterRoleObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteSecretObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteRoleBindingObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteClusterRoleBindingObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteConfigmapObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) deleteValidatingWebhookObject(val boundlessv1alpha1.ManifestObject, ctx context.Context) error {
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

func (r *ManifestReconciler) UpdateManifestObjects(req ctrl.Request, ctx context.Context, existing *boundlessv1alpha1.Manifest) error {
	logger := log.FromContext(ctx)

	// Read the URL contents
	bodyBytes, err := r.ReadManifest(req, existing.Spec.Url, logger)
	if err != nil {
		logger.Error(err, "failed to fetch manifest file content for url: %s", existing.Spec.Url)
		return err
	}

	decoder := clientgoscheme.Codecs.UniversalDeserializer()

	newManifestObjs := []boundlessv1alpha1.ManifestObject{}
	val := boundlessv1alpha1.ManifestObject{}

	for _, obj := range strings.Split(string(bodyBytes), "---") {
		if obj != "" {
			runtimeObject, groupVersionKind, err := decoder.Decode([]byte(obj), nil, nil)
			if err != nil {
				logger.Info("Failed to decode yaml:", "Error", err)
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			switch groupVersionKind.Kind {
			case "Namespace":
				err := r.handleNamespaceObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "Service":
				err := r.handleServiceObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "Deployment":
				err := r.handleDeploymentObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "DaemonSet":
				err := r.handleDaemonsetObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "PodDisruptionBudget":
				err := r.handlePodDisruptionBudget(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "ServiceAccount":
				err := r.handleServiceAccount(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "Role":
				err := r.handleRoleObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "ClusterRole":
				err := r.handleClusterRoleObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "Secret":
				err := r.handleSecretObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "RoleBinding":
				err := r.handleRoleBindingObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "ClusterRoleBinding":
				err := r.handleClusterRoleBindingObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "ConfigMap":
				err := r.handleConfigMapObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "CustomResourceDefinition":
				err := r.handleCRDObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			case "ValidatingWebhookConfiguration":
				err := r.handleValidatingWebhookObject(runtimeObject, groupVersionKind, req, ctx, &newManifestObjs, actionUpdate, val)
				if err != nil {
					return err
				}

			default:
				logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
			}
		}
	}

	// Get the list of old objects
	oldObjects := existing.Spec.Objects

	// Update the CRD
	key := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	crd := &boundlessv1alpha1.Manifest{}
	err = r.Client.Get(ctx, key, crd)
	if err != nil {
		logger.Error(err, "failed to get manifest object")
		return err
	}
	updatedCRD := boundlessv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            crd.Name,
			Namespace:       crd.Namespace,
			ResourceVersion: crd.ResourceVersion,
		},
		Spec: boundlessv1alpha1.ManifestSpec{
			Url:         crd.Spec.Url,
			Checksum:    crd.Spec.NewChecksum,
			NewChecksum: crd.Spec.NewChecksum,
			Objects:     newManifestObjs,
		},
	}

	if err := r.Update(ctx, &updatedCRD); err != nil {
		logger.Error(err, "failed to update manifest crd with objectList during update operation")
		return err
	}

	// Find the intersection of the new manifest based
	// objects and old manifest based objects and delete the extra.
	r.findAndDeleteObsoleteObjects(req, ctx, oldObjects, newManifestObjs)

	return nil
}

// TODO: https://github.com/Mirantis/boundless-operator/pull/17#discussion_r1408571732
func (r *ManifestReconciler) findAndDeleteObsoleteObjects(req ctrl.Request, ctx context.Context, oldObjects []boundlessv1alpha1.ManifestObject, newObjects []boundlessv1alpha1.ManifestObject) {
	logger := log.FromContext(ctx)

	obsolete := []boundlessv1alpha1.ManifestObject{}

	if len(oldObjects) > 0 && len(newObjects) > 0 {
		for _, old := range oldObjects {
			found := false
			for _, new := range newObjects {
				if reflect.DeepEqual(old, new) {
					found = true
					break
				}
			}

			if found == false {
				logger.Info("obsolete object found", "Name", old.Name, "Kind", old.Kind)
				obsolete = append(obsolete, old)
			}

		}

		if err := r.DeleteManifestObjects(obsolete, ctx); err != nil {
			logger.Error(err, "failed to delete obsolete objects")
		}
	}

}

func (r *ManifestReconciler) ReadManifest(req ctrl.Request, url string, logger logr.Logger) ([]byte, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.Error(err, "failed to create http request for url: %s", url)
		return nil, err
	}

	client := http.DefaultClient

	resp, err := client.Do(httpReq)
	if err != nil {
		logger.Error(err, "failed to fetch manifest file content for url: %s", url)
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
		return nil, fmt.Errorf("failure in http get request ResponseCode: %d, %s", resp.StatusCode, err)
	}

	return bodyBytes, nil

}

func (r *ManifestReconciler) updateStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, typeToApply boundlessv1alpha1.StatusType, reasonToApply string, messageToApply ...string) error {
	manifest := &boundlessv1alpha1.Manifest{}
	err := r.Get(ctx, namespacedName, manifest)
	if err != nil {
		logger.Error(err, "Failed to get manifest to update status")
		return err
	}

	if manifest.Status.Type == typeToApply && manifest.Status.Reason == reasonToApply {
		// avoid infinite reconciliation loops
		logger.Info("No updates to status needed")
		return nil
	}

	logger.Info("Update status for manifest", "Name", manifest.Name)

	patch := client.MergeFrom(manifest.DeepCopy())
	manifest.Status.Type = typeToApply
	manifest.Status.Reason = reasonToApply
	if len(messageToApply) > 0 {
		manifest.Status.Message = messageToApply[0]
	}
	manifest.Status.LastTransitionTime = metav1.Now()

	return r.Status().Patch(ctx, manifest, patch)
}
