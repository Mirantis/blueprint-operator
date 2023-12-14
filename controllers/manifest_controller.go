package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apps_v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/event"
	"github.com/mirantis/boundless-operator/pkg/kubernetes"
)

const (
	actionUpdate        = "update"
	actionCreate        = "create"
	actionDelete        = "delete"
	manifestUpdateIndex = "manifestupdateindex"
)

// ManifestReconciler reconciles a Manifest object
type ManifestReconciler struct {
	client.Client
	dynamic.DynamicClient
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=boundless.mirantis.com,resources=manifests/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=daemonsets/status,verbs=get

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
			if err := r.DeleteManifestObjects(ctx, existing.Spec.Objects); err != nil {
				logger.Error(err, "failed to delete manifest objects")
				r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedDelete, "failed to delete manifest objects %s/%s", existing.Namespace, existing.Name)
				r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to delete manifest objects", fmt.Sprintf("failed to delete manifest objects : %s", err))
				return ctrl.Result{Requeue: true}, err
			}

			// Remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(existing, addonFinalizerName)
			if err := r.Update(ctx, existing); err != nil {
				logger.Error(err, "failed to remove finalizer")
				r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonSuccessfulCreate, "failed to remove finalizer %s/%s", existing.Namespace, existing.Name)
				r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to remove finalizer", fmt.Sprintf("failed to remove finalizer : %s", err))
				return ctrl.Result{Requeue: true}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if existing.Spec.Checksum == existing.Spec.NewChecksum {
		logger.Info("checksum is same, no update needed", "Checksum", existing.Spec.Checksum, "NewChecksum", existing.Spec.NewChecksum)
		// manifest is already installed as specified - get latest status from objects in the cluster
		err = r.checkManifestStatus(ctx, logger, req.NamespacedName, existing.Spec.Objects)
		return ctrl.Result{}, err
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
			r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to update manifest crd while update operation ", fmt.Sprintf("failed to update manifest crd while update operation  : %s", err))
			return ctrl.Result{}, err
		}

		// TODO: https://github.com/mirantiscontainers/boundless-operator/pull/17#pullrequestreview-1754136032
		if err := r.UpdateManifestObjects(req, ctx, existing); err != nil {
			logger.Error(err, "failed to update manifest")
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			r.updateStatus(ctx, logger, key, boundlessv1alpha1.TypeComponentUnhealthy, "failed to update manifest ", fmt.Sprintf("failed to update manifest  : %s", err))
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
			return ctrl.Result{}, err
		}

		// Run http get request to fetch the contents of the manifest file.
		bodyBytes, err := r.ReadManifest(req, existing.Spec.Url, logger)
		if err != nil {
			logger.Error(err, "failed to fetch manifest file content for url: %s", "Manifest Url", existing.Spec.Url)
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to fetch manifest file content for url %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}

		logger.Info("received new crd request. Creating manifest objects..")
		err = r.CreateManifestObjects(ctx, req, logger, bodyBytes)
		if err != nil {
			logger.Error(err, "failed to create objects for the manifest", "Name", req.Name)
			r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to create objects for the manifest %s/%s : %s", existing.Namespace, existing.Name, err.Error())
			return ctrl.Result{}, err
		}

	}

	r.Recorder.AnnotatedEventf(existing, map[string]string{event.AddonAnnotationKey: existing.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Manifest %s/%s", existing.Namespace, existing.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// attaches an index onto the Manifest
	// This is done so we can later easily find the addon associated with a particular deployment or daemonset
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &boundlessv1alpha1.Manifest{}, manifestUpdateIndex, func(rawObj client.Object) []string {
		manifest := rawObj.(*boundlessv1alpha1.Manifest)
		if manifest.Spec.Objects == nil || len(manifest.Spec.Objects) == 0 {
			return nil
		}

		var indexes []string
		for _, obj := range manifest.Spec.Objects {
			if obj.Kind == "DaemonSet" || obj.Kind == "Deployment" {
				indexes = append(indexes, fmt.Sprintf("%s-%s", obj.Namespace, obj.Name))
			}
		}
		return indexes

	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&boundlessv1alpha1.Manifest{}).
		Watches(
			&apps_v1.DaemonSet{},
			handler.EnqueueRequestsFromMapFunc(r.findAssociatedManifest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&apps_v1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.findAssociatedManifest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

// findAssociatedManifest finds the manifest tied to a particular object if one exists
// This is done by looking for the manifest that was previously indexed in the form objectNamespace-objectName
func (r *ManifestReconciler) findAssociatedManifest(ctx context.Context, obj client.Object) []reconcile.Request {
	attachedManifestList := &boundlessv1alpha1.ManifestList{}
	//TODO: this index will clash if we have multiple deployments / daemonsets with the same name and namespace
	err := r.List(context.TODO(), attachedManifestList, client.MatchingFields{manifestUpdateIndex: fmt.Sprintf("%s-%s", obj.GetNamespace(), obj.GetName())})
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(attachedManifestList.Items))
	for i, item := range attachedManifestList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

func (r *ManifestReconciler) CreateManifestObjects(ctx context.Context, req ctrl.Request, logger logr.Logger, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := kubernetes.Apply(ctx, logger, r.Client, data); err != nil {
		return err
	}

	objs, err := decodeObjects(data)
	if err != nil {
		return err
	}
	var manifestObjs []boundlessv1alpha1.ManifestObject
	for _, o := range objs {
		manifestObjs = append(manifestObjs, boundlessv1alpha1.ManifestObject{
			Group:     o.GroupVersionKind().Group,
			Version:   o.GroupVersionKind().Version,
			Kind:      o.GetKind(),
			Name:      o.GetName(),
			Namespace: o.GetNamespace(),
		})
	}

	// TODO: https://github.com/mirantiscontainers/boundless-operator/pull/17#discussion_r1408570381
	// Update the CRD
	key := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	crd := &boundlessv1alpha1.Manifest{}
	if err = r.Client.Get(ctx, key, crd); err != nil {
		logger.Error(err, "failed to get manifest resource %s/%s", req.Namespace, req.Namespace)
		return fmt.Errorf("failed to get manifest resource %s/%s: %w", req.Namespace, req.Namespace, err)
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

	if err = r.Update(ctx, &updatedCRD); err != nil {
		logger.Error(err, "failed to update manifest crd with objectList during create")
		return err
	}

	return nil
}

func (r *ManifestReconciler) DeleteManifestObjects(ctx context.Context, objectList []boundlessv1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	var objs []unstructured.Unstructured
	for _, item := range objectList {
		u := unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   item.Group,
			Version: item.Version,
			Kind:    item.Kind,
		})
		u.SetName(item.Name)
		u.SetNamespace(item.Namespace)
		objs = append(objs, u)
	}
	if err := kubernetes.Delete(ctx, logger, r.Client, objs); err != nil {
		return fmt.Errorf("failed to delete objects for manifest: %w", err)
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

	if err = kubernetes.Apply(ctx, logger, r.Client, bodyBytes); err != nil {
		return err
	}
	// Get the list of old objects
	oldObjects := existing.Spec.Objects

	objs, err := decodeObjects(bodyBytes)
	if err != nil {
		return err
	}
	var newManifestObjs []boundlessv1alpha1.ManifestObject
	for _, o := range objs {
		newManifestObjs = append(newManifestObjs, boundlessv1alpha1.ManifestObject{
			Group:     o.GroupVersionKind().Group,
			Version:   o.GroupVersionKind().Version,
			Kind:      o.GetKind(),
			Name:      o.GetName(),
			Namespace: o.GetNamespace(),
		})
	}

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

// TODO: https://github.com/mirantiscontainers/boundless-operator/pull/17#discussion_r1408571732
func (r *ManifestReconciler) findAndDeleteObsoleteObjects(req ctrl.Request, ctx context.Context, oldObjects []boundlessv1alpha1.ManifestObject, newObjects []boundlessv1alpha1.ManifestObject) {
	logger := log.FromContext(ctx)

	var obsolete []boundlessv1alpha1.ManifestObject

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

		if err := r.DeleteManifestObjects(ctx, obsolete); err != nil {
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

// checkManifestStatus checks the status of any deployments and daemonsets associated with the namespacedName manifest
// Check the status of the deployment and daemonset and set the manifest to an error state if any errors are found
// If no errors are found, we check if any deployments/daemonsets are still progressing and set the manifest status to Progressing
// Otherwise set the manifest status to Available
// This is not comprehensive and may need to be updated as we support more complex manifests
func (r *ManifestReconciler) checkManifestStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, objects []boundlessv1alpha1.ManifestObject) error {

	if objects == nil || len(objects) == 0 {
		logger.Info("No manifest objects for manifest")
		return nil
	}

	// for now focus on getting status from any Deployments or Daemonsets deployed via the manifest since
	// they have reliable status fields we can pull from and are most likely to fail
	stillProgressing := false
	var reasonToApply, messageToApply string
	for _, obj := range objects {
		kind := obj.Kind

		if kind == "Deployment" {
			deployment := &apps_v1.Deployment{}
			err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}, deployment)
			if err != nil {
				return err
			}
			if deployment.Status.AvailableReplicas == deployment.Status.Replicas && (deployment.Status.Conditions == nil || len(deployment.Status.Conditions) == 0) {
				// this deployment is ready
				continue
			}
			latestCondition := deployment.Status.Conditions[0]
			if deployment.Status.AvailableReplicas == deployment.Status.Replicas && latestCondition.Type == apps_v1.DeploymentAvailable {
				// this deployment is ready
				continue
			}

			if latestCondition.Type == apps_v1.DeploymentProgressing || latestCondition.Reason == "MinimumReplicasUnavailable" {
				stillProgressing = true
				reasonToApply = fmt.Sprintf("Deployment %s still progressing", obj.Name)
				messageToApply = latestCondition.Message
			} else {
				// deployment is in error state, so we can update the manifest status that it has issues
				err := r.updateStatus(ctx, logger, namespacedName, boundlessv1alpha1.TypeComponentUnhealthy, latestCondition.Reason, latestCondition.Message)
				if err != nil {
					return err
				}
				break
			}
		} else if kind == "DaemonSet" {
			daemonset := &apps_v1.DaemonSet{}
			err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}, daemonset)
			if err != nil {
				return err
			}

			if daemonset.Status.DesiredNumberScheduled == daemonset.Status.CurrentNumberScheduled && daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberAvailable {
				//daemonset is ready
				continue
			}

			if daemonset.Status.NumberMisscheduled > 0 {
				err := r.updateStatus(ctx, logger, namespacedName, boundlessv1alpha1.TypeComponentUnhealthy, fmt.Sprintf("Daemonset %s failed to schedule pods", daemonset.Name))
				if err != nil {
					return err
				}
				break
			} else {
				stillProgressing = true
				reasonToApply = fmt.Sprintf("Daemonset %s is still progressing", daemonset.Name)
				messageToApply = fmt.Sprintf("Daemonset %s is still progressing", daemonset.Name)
			}

		} else {
			continue
		}

	}

	if stillProgressing {
		err := r.updateStatus(ctx, logger, namespacedName, boundlessv1alpha1.TypeComponentProgressing, fmt.Sprintf("One or more components still progressing : %s", reasonToApply), messageToApply)
		if err != nil {
			return err
		}
		return nil
	}

	err := r.updateStatus(ctx, logger, namespacedName, boundlessv1alpha1.TypeComponentAvailable, "Manifest Components Available", "Manifest Components Available")
	if err != nil {
		return err
	}

	return nil
}

// updateStatus queries for a fresh Manifest with the provided namespacedName.
// It then updates the Manifest's status fields with the provided type, reason, and optionally message.
func (r *ManifestReconciler) updateStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, typeToApply boundlessv1alpha1.StatusType, reasonToApply string, messageToApply ...string) error {
	logger.Info("Update status with type and reason", "TypeToApply", typeToApply, "ReasonToApply", reasonToApply)

	manifest := &boundlessv1alpha1.Manifest{}
	err := r.Get(ctx, namespacedName, manifest)
	if err != nil {
		logger.Error(err, "Failed to get manifest to update status")
		return err
	}

	nilStatus := boundlessv1alpha1.ManifestStatus{}
	if manifest.Status != nilStatus && manifest.Status.Type == typeToApply && manifest.Status.Reason == reasonToApply {
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

func decodeObjects(data []byte) ([]unstructured.Unstructured, error) {
	var objs []unstructured.Unstructured
	decoder := yaml.NewYAMLToJSONDecoder(bytes.NewReader(data))

	var o unstructured.Unstructured
	for {
		if err := decoder.Decode(&o); err != nil {
			if err != io.EOF {
				return objs, fmt.Errorf("error decoding yaml manifest file: %s", err)
			}
			break
		}
		objs = append(objs, o)

	}
	return objs, nil
}
