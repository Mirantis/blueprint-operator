package controllers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mirantiscontainers/blueprint-operator/client/api/v1alpha1"
	"io"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pkgmanifest "github.com/mirantiscontainers/blueprint-operator/pkg/controllers/manifest"
	"github.com/mirantiscontainers/blueprint-operator/pkg/event"
	"github.com/mirantiscontainers/blueprint-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/blueprint-operator/pkg/kustomize"
)

const (
	manifestUpdateIndex = "manifestupdateindex"
)

// ManifestReconciler reconciles a Manifest object
type ManifestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=manifests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=manifests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=blueprint.mirantis.com,resources=manifests/finalizers,verbs=update
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
	start := time.Now()
	var err error
	defer func() {
		ManifestHistVec.WithLabelValues(req.Name, getMetricStatus(err)).Observe(time.Since(start).Seconds())
	}()

	key := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	instance := &v1alpha1.Manifest{}
	if err = r.Client.Get(ctx, key, instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Manifest instance not found. Ignoring since object must be deleted.", "Name", req.Name)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Manifest instance", "Name", req.Name, "Requeue", true)
		return ctrl.Result{}, err
	}

	finalizerName := "manifest/finalizer"
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, finalizerName) {
			controllerutil.AddFinalizer(instance, finalizerName)
			if err := r.Update(ctx, instance); err != nil {
				logger.Info("failed to update manifest object with finalizer", "Name", req.Name, "Finalizer", finalizerName)
				r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest object with finalizer %s/%s", instance.Namespace, instance.Name)
				r.updateStatus(ctx, logger, key, v1alpha1.TypeComponentUnhealthy, "failed to update manifest object with finalizer", fmt.Sprintf("failed to update manifest object with finalizer : %s", err))
				return ctrl.Result{}, err
			}
			logger.Info("finalizer added successfully", "Name", req.Name, "Finalizer", finalizerName)
			return ctrl.Result{}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(instance, finalizerName) {
			// The finalizer is present, so let's delete the objects for this manifest
			if err = r.DeleteManifestObjects(ctx, instance.Spec.Objects); err != nil {
				logger.Error(err, "failed to delete manifest objects")
				r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedDelete, "failed to delete manifest objects %s/%s", instance.Namespace, instance.Name)
				r.updateStatus(ctx, logger, key, v1alpha1.TypeComponentUnhealthy, "failed to delete manifest objects", fmt.Sprintf("failed to delete manifest objects : %s", err))
				return ctrl.Result{}, err
			}

			// Remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(instance, finalizerName)
			if err = r.Update(ctx, instance); err != nil {
				logger.Error(err, "failed to remove finalizer")
				r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonSuccessfulCreate, "failed to remove finalizer %s/%s", instance.Namespace, instance.Name)
				r.updateStatus(ctx, logger, key, v1alpha1.TypeComponentUnhealthy, "failed to remove finalizer", fmt.Sprintf("failed to remove finalizer : %s", err))
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if instance.Spec.Checksum == instance.Spec.NewChecksum {
		logger.Info("checksum is same, no update needed", "Checksum", instance.Spec.Checksum, "NewChecksum", instance.Spec.NewChecksum)

		if pkgmanifest.ShouldRetryManifest(logger, instance) {
			logger.Info("Reapplying manifest")
			// wipe the manifest checksum to get reconcile to run an Update
			instance.Spec.Checksum = ""
			if err = r.Update(ctx, instance); err != nil {
				logger.Error(err, "failed to wipe checksum for manifest")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// manifest is already installed as specified - update manifest status from status's of objects in the cluster
		if err = r.updateManifestStatus(ctx, logger, req.NamespacedName, instance.Spec.Objects); err != nil {
			logger.Error(err, "failed to update manifest status")
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest status %s/%s : %s", instance.Namespace, instance.Name, err.Error())
			r.updateStatus(ctx, logger, key, v1alpha1.TypeComponentUnhealthy, "failed to update manifest status", fmt.Sprintf("failed to update manifest status : %s", err))
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if (instance.Spec.Checksum != instance.Spec.NewChecksum) && (instance.Spec.NewChecksum != "") {
		// Update is required
		logger.Info("checksum differs, update needed", "Checksum", instance.Spec.Checksum, "NewChecksum", instance.Spec.NewChecksum)
		// First, update the checksum to avoid any reconciliation
		// Update the CRD
		// @todo (Ranyodh): The CRD should also add finalizer (or do a Patch() update), otherwise, the finalizer will be removed
		updatedCRD := v1alpha1.Manifest{
			ObjectMeta: metav1.ObjectMeta{
				Name:            instance.Name,
				Namespace:       instance.Namespace,
				ResourceVersion: instance.ResourceVersion,
			},
			Spec: v1alpha1.ManifestSpec{
				Url:           instance.Spec.Url,
				Checksum:      instance.Spec.NewChecksum,
				NewChecksum:   instance.Spec.NewChecksum,
				FailurePolicy: instance.Spec.FailurePolicy,
				Timeout:       instance.Spec.Timeout,
				Values:        instance.Spec.Values,
			},
		}

		// @TODO Ranyodh: The update to CRD here will trigger a new reconcile, while this current reconcile will continue to process
		// This will cause errors as the manifest is already has been updated.
		// Also, the call to  UpdateManifestObjects() after this causes the CRD to be updated again.
		// There should be only one reconcile for the update of the manifest. This needs to be fixed.
		if err = r.Update(ctx, &updatedCRD); err != nil {
			logger.Error(err, "failed to update manifest crd while update operation")
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest resource while update operation %s/%s : %s", instance.Namespace, instance.Name, err.Error())
			r.updateStatus(ctx, logger, key, v1alpha1.TypeComponentUnhealthy, "failed to update manifest crd while update operation ", fmt.Sprintf("failed to update manifest crd while update operation  : %s", err))
			return ctrl.Result{}, err
		}

		// TODO: https://github.com/mirantiscontainers/blueprint-operator/pull/17#pullrequestreview-1754136032
		if err = r.UpdateManifestObjects(req, ctx, instance); err != nil {
			logger.Error(err, "failed to update manifest")
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest %s/%s : %s", instance.Namespace, instance.Name, err.Error())
			r.updateStatus(ctx, logger, key, v1alpha1.TypeComponentUnhealthy, "failed to update manifest ", fmt.Sprintf("failed to update manifest  : %s", err))
			return ctrl.Result{}, err
		}

		if instance.Spec.Timeout != "" && instance.Spec.FailurePolicy == pkgmanifest.FailurePolicyRetry {
			var timeoutDuration time.Duration
			timeoutDuration, err = time.ParseDuration(instance.Spec.Timeout)
			if err != nil {
				logger.Error(err, "failed to parse timeout for manifest", "Timeout", timeoutDuration)
				r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to parse timeout for the manifest %s/%s : %s", instance.Namespace, instance.Name, err.Error())
				return ctrl.Result{}, err
			}
			go r.retryUpgradeInstallAfterTimeout(ctx, logger, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, timeoutDuration, instance.Spec.FailurePolicy, false)
		}
	}

	if instance.Spec.NewChecksum == "" {
		// We will reach here only in case of create request.
		// First, update the checksum in CRD to avoid any reconciliations.
		// Update the CRD
		// @todo (Ranyodh): The CRD should also add finalizer (or do a Patch() update), otherwise, the finalizer will be removed
		updatedCRD := v1alpha1.Manifest{
			ObjectMeta: metav1.ObjectMeta{
				Name:            instance.Name,
				Namespace:       instance.Namespace,
				ResourceVersion: instance.ResourceVersion,
			},
			Spec: v1alpha1.ManifestSpec{
				Url:           instance.Spec.Url,
				Checksum:      instance.Spec.Checksum,
				NewChecksum:   instance.Spec.Checksum,
				Timeout:       instance.Spec.Timeout,
				FailurePolicy: instance.Spec.FailurePolicy,
				Values:        instance.Spec.Values,
			},
		}

		// @TODO Ranyodh: The update to CRD here will trigger a new reconcile. We must exit the reconciler after this.
		if err = r.Update(ctx, &updatedCRD); err != nil {
			logger.Error(err, "failed to update manifest crd while create operation")
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to update manifest crd while create operation %s/%s : %s", instance.Namespace, instance.Name, err.Error())
			return ctrl.Result{}, err
		}

		// Create the kustomize file, get kustomize build output and create objects thereby.
		var bodyBytes []byte
		bodyBytes, err = kustomize.Render(logger, instance.Spec.Url, instance.Spec.Values)

		if err != nil {
			logger.Error(err, "failed to fetch manifest file content for url: %s", "Manifest Url", instance.Spec.Url)
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to fetch manifest file content for url %s/%s : %s", instance.Namespace, instance.Name, err.Error())
			return ctrl.Result{}, err
		}

		logger.Info("received new crd request. Creating manifest objects..")
		err = r.CreateManifestObjects(ctx, key, logger, bodyBytes)
		if err != nil {
			logger.Error(err, "failed to create objects for the manifest", "Name", req.Name)
			r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to create objects for the manifest %s/%s : %s", instance.Namespace, instance.Name, err.Error())
			return ctrl.Result{}, err
		}

		if instance.Spec.Timeout != "" && instance.Spec.FailurePolicy != pkgmanifest.FailurePolicyNone {
			var timeoutDuration time.Duration
			timeoutDuration, err = time.ParseDuration(instance.Spec.Timeout)
			if err != nil {
				logger.Error(err, "failed to parse timeout for manifest", "Timeout", timeoutDuration)
				r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeWarning, event.ReasonFailedCreate, "failed to parse timeout for the manifest %s/%s : %s", instance.Namespace, instance.Name, err.Error())
				return ctrl.Result{}, err
			}
			go r.retryUpgradeInstallAfterTimeout(ctx, logger, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, timeoutDuration, instance.Spec.FailurePolicy, true)
		}
	}
	r.Recorder.AnnotatedEventf(instance, map[string]string{event.AddonAnnotationKey: instance.Name}, event.TypeNormal, event.ReasonSuccessfulCreate, "Created Manifest %s/%s", instance.Namespace, instance.Name)
	return ctrl.Result{}, nil
}

// retryUpgradeInstallAfterTimeout checks if the manifest is Available after Timeout, and if it is not then it retries the upgrade/install.
func (r *ManifestReconciler) retryUpgradeInstallAfterTimeout(ctx context.Context, logger logr.Logger, manifestName types.NamespacedName, timeout time.Duration, failurePolicy string, isInstall bool) {

	mc := pkgmanifest.NewManifestController(r.Client, logger)
	timeoutErr := mc.AwaitTimeout(logger, manifestName, timeout)
	if timeoutErr != nil {
		// manifest is not available before timeout
		var manifest v1alpha1.Manifest
		err := r.Get(ctx, manifestName, &manifest)
		if err != nil {
			logger.Error(err, "Failed to get manifest")
			return
		}

		r.Recorder.AnnotatedEventf(&manifest, map[string]string{event.AddonAnnotationKey: manifest.Name}, event.TypeWarning, event.ReasonFailedCreate, "manifest creation timed out %s/%s : %s", manifest.Namespace, manifest.Name, timeoutErr.Error())

		if isInstall {
			// if it's an install then delete existing manifest objects so they can be fully re-installed

			logger.Info("Deleting manifest objects ", "ManifestName", manifestName)
			err = r.DeleteManifestObjects(ctx, manifest.Spec.Objects)
			if err != nil {
				logger.Error(err, "Failed to delete manifest objects")
				return
			}

		}

		// wipe the manifest checksum to get reconcile to run an Update
		manifest.Spec.Checksum = ""
		if err = r.Update(ctx, &manifest); err != nil {
			logger.Error(err, "failed to wipe checksum for manifest")
		}
		return
	}

	logger.Info("Manifest is Available before Timeout", "ManifestName", manifestName)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// attaches an index onto the Manifest
	// This is done, so we can later easily find the addon associated with a particular deployment or daemonset
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.Manifest{}, manifestUpdateIndex, func(rawObj client.Object) []string {
		manifest := rawObj.(*v1alpha1.Manifest)
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
		For(&v1alpha1.Manifest{}).
		Watches(
			&appsv1.DaemonSet{},
			handler.EnqueueRequestsFromMapFunc(r.findAssociatedManifest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.findAssociatedManifest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

// findAssociatedManifest finds the manifest tied to a particular object if one exists
// This is done by looking for the manifest that was previously indexed in the form objectNamespace-objectName
func (r *ManifestReconciler) findAssociatedManifest(ctx context.Context, obj client.Object) []reconcile.Request {
	attachedManifestList := &v1alpha1.ManifestList{}
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

// CreateManifestObjects reads manifest from a url and then create all objects in the cluster
func (r *ManifestReconciler) CreateManifestObjects(ctx context.Context, manifestNamespacedName types.NamespacedName, logger logr.Logger, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(logger, r.Client)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader(data)); err != nil {
		return err
	}

	objs, err := decodeObjects(data)
	if err != nil {
		return err
	}
	var manifestObjs []v1alpha1.ManifestObject
	for _, o := range objs {
		manifestObjs = append(manifestObjs, v1alpha1.ManifestObject{
			Group:     o.GroupVersionKind().Group,
			Version:   o.GroupVersionKind().Version,
			Kind:      o.GetKind(),
			Name:      o.GetName(),
			Namespace: o.GetNamespace(),
		})
	}

	// TODO: https://github.com/mirantiscontainers/blueprint-operator/pull/17#discussion_r1408570381
	// Update the CRD

	crd := &v1alpha1.Manifest{}
	if err = r.Client.Get(ctx, manifestNamespacedName, crd); err != nil {
		logger.Error(err, "failed to get manifest resource %s/%s", manifestNamespacedName.Namespace, manifestNamespacedName.Namespace)
		return fmt.Errorf("failed to get manifest resource %s/%s: %w", manifestNamespacedName.Namespace, manifestNamespacedName.Namespace, err)
	}
	// Update the CRD
	updatedCRD := v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            crd.Name,
			Namespace:       crd.Namespace,
			ResourceVersion: crd.ResourceVersion,
		},
		Spec: v1alpha1.ManifestSpec{
			Url:           crd.Spec.Url,
			Checksum:      crd.Spec.Checksum,
			NewChecksum:   crd.Spec.NewChecksum,
			FailurePolicy: crd.Spec.FailurePolicy,
			Timeout:       crd.Spec.Timeout,
			Objects:       manifestObjs,
		},
	}

	if err = r.Update(ctx, &updatedCRD); err != nil {
		logger.Error(err, "failed to update manifest crd with objectList during create")
		return err
	}

	return nil
}

func (r *ManifestReconciler) DeleteManifestObjects(ctx context.Context, objectList []v1alpha1.ManifestObject) error {
	logger := log.FromContext(ctx)

	var objs []*unstructured.Unstructured
	for _, item := range objectList {
		u := unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   item.Group,
			Version: item.Version,
			Kind:    item.Kind,
		})
		u.SetName(item.Name)
		u.SetNamespace(item.Namespace)
		objs = append(objs, &u)
	}

	applier := kubernetes.NewApplier(logger, r.Client)
	if err := applier.Delete(ctx, objs); err != nil {
		return fmt.Errorf("failed to delete objects for manifest: %w", err)
	}
	return nil
}

// UpdateManifestObjects reads the manifest from a url and then create or update the new/existing objects in the cluster
func (r *ManifestReconciler) UpdateManifestObjects(req ctrl.Request, ctx context.Context, existing *v1alpha1.Manifest) error {
	logger := log.FromContext(ctx)

	// Create kustomize file, generate kustomize build output and update the objects.
	bodyBytes, err := kustomize.Render(logger, existing.Spec.Url, existing.Spec.Values)

	if err != nil {
		logger.Error(err, "failed to fetch manifest file content for url: %s", existing.Spec.Url)
		return err
	}

	applier := kubernetes.NewApplier(logger, r.Client)

	if err = applier.Apply(ctx, kubernetes.NewManifestReader(bodyBytes)); err != nil {
		return err
	}
	// Get the list of old objects
	oldObjects := existing.Spec.Objects

	objs, err := decodeObjects(bodyBytes)
	if err != nil {
		return err
	}
	var newManifestObjs []v1alpha1.ManifestObject
	for _, o := range objs {
		newManifestObjs = append(newManifestObjs, v1alpha1.ManifestObject{
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

	crd := &v1alpha1.Manifest{}
	err = r.Client.Get(ctx, key, crd)
	if err != nil {
		logger.Error(err, "failed to get manifest object")
		return err
	}

	// @todo (Ranyodh): The CRD should also add finalizer (or do a Patch() update), otherwise, the finalizer will be removed
	updatedCRD := v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            crd.Name,
			Namespace:       crd.Namespace,
			ResourceVersion: crd.ResourceVersion,
		},
		Spec: v1alpha1.ManifestSpec{
			Url:           crd.Spec.Url,
			Checksum:      crd.Spec.NewChecksum,
			NewChecksum:   crd.Spec.NewChecksum,
			FailurePolicy: crd.Spec.FailurePolicy,
			Timeout:       crd.Spec.Timeout,
			Objects:       newManifestObjs,
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

// TODO: https://github.com/mirantiscontainers/blueprint-operator/pull/17#discussion_r1408571732
func (r *ManifestReconciler) findAndDeleteObsoleteObjects(req ctrl.Request, ctx context.Context, oldObjects []v1alpha1.ManifestObject, newObjects []v1alpha1.ManifestObject) {
	logger := log.FromContext(ctx)

	var obsolete []v1alpha1.ManifestObject

	if len(oldObjects) > 0 && len(newObjects) > 0 {
		for _, old := range oldObjects {
			found := false
			for _, n := range newObjects {
				if reflect.DeepEqual(old, n) {
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

func (r *ManifestReconciler) updateManifestStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, objects []v1alpha1.ManifestObject) error {
	mc := pkgmanifest.NewManifestController(r.Client, logger)
	manifestStatus, err := mc.CheckManifestStatus(ctx, logger, objects)
	if err != nil {
		return err
	}
	err = r.updateStatus(ctx, logger, namespacedName, manifestStatus.StatusType, manifestStatus.Reason, manifestStatus.Message)
	if err != nil {
		return err
	}

	return nil
}

// updateStatus queries for a fresh Manifest with the provided namespacedName.
// It then updates the Manifest's status fields with the provided type, reason, and optionally message.
func (r *ManifestReconciler) updateStatus(ctx context.Context, logger logr.Logger, namespacedName types.NamespacedName, typeToApply v1alpha1.StatusType, reasonToApply string, messageToApply ...string) error {
	logger.Info("Update status with type and reason", "TypeToApply", typeToApply, "ReasonToApply", reasonToApply)

	manifest := &v1alpha1.Manifest{}
	err := r.Get(ctx, namespacedName, manifest)
	if err != nil {
		logger.Error(err, "Failed to get manifest to update status")
		return err
	}

	nilStatus := v1alpha1.ManifestStatus{}
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
