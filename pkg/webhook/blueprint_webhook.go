package webhook

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
)

const (
	kindManifest = "manifest"
	kindChart    = "chart"
)

// log is for logging in this package.
var blueprintlog = logf.Log.WithName("blueprint-resource")

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Blueprint{}).
		WithDefaulter(&blueprintDefaulter{}).
		WithValidator(&blueprintValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-blueprint-mirantis-com-v1alpha1-blueprint,mutating=true,failurePolicy=fail,sideEffects=None,groups=blueprint.mirantis.com,resources=blueprints,verbs=create;update,versions=v1alpha1,name=mblueprint.kb.io,admissionReviewVersions=v1

type blueprintDefaulter struct{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *blueprintDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	blueprint, ok := obj.(*v1alpha1.Blueprint)
	if !ok {
		return fmt.Errorf("obj %v is not a blueprint kind", obj.GetObjectKind())
	}
	blueprintlog.Info("default", "name", blueprint.Name)
	return nil
}

// change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-blueprint-mirantis-com-v1alpha1-blueprint,mutating=false,failurePolicy=fail,sideEffects=None,groups=blueprint.mirantis.com,resources=blueprints,verbs=create;update,versions=v1alpha1,name=vblueprint.kb.io,admissionReviewVersions=v1

type blueprintValidator struct{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *blueprintValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	blueprint, ok := obj.(*v1alpha1.Blueprint)
	if !ok {
		return nil, fmt.Errorf("obj %v is not a blueprint kind", obj.GetObjectKind())
	}
	blueprintlog.Info("validate create", "name", blueprint.Name)
	return validate(blueprint.Spec)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *blueprintValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	blueprint, ok := newObj.(*v1alpha1.Blueprint)
	if !ok {
		return nil, fmt.Errorf("obj %v is not a blueprint kind", newObj.GetObjectKind())
	}
	blueprintlog.Info("validate update", "name", blueprint.Name)
	return validate(blueprint.Spec)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *blueprintValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	blueprint, ok := obj.(*v1alpha1.Blueprint)
	if !ok {
		return nil, fmt.Errorf("obj %v is not a blueprint kind", obj.GetObjectKind())
	}
	blueprintlog.Info("validate delete", "name", blueprint.Name)

	return nil, nil
}

func validate(spec v1alpha1.BlueprintSpec) (admission.Warnings, error) {
	if len(spec.Components.Addons) == 0 {
		return nil, nil
	}

	var addonNames []string
	for _, a := range spec.Components.Addons {
		addonNames = append(addonNames, a.Name)
	}

	for _, val := range spec.Components.Addons {
		if strings.EqualFold(kindChart, val.Kind) {
			if val.Manifest != nil {
				blueprintlog.Info("received manifest object.", "Kind", kindChart)
				return nil, fmt.Errorf("manifest object is not allowed for addon kind %s", kindChart)
			}
			if val.Chart == nil {
				blueprintlog.Info("received empty chart object.", "Kind", kindChart)
				return nil, fmt.Errorf("chart object can't be empty for addon kind %s", kindChart)
			}
			if len(val.Chart.DependsOn) > 0 {
				for _, dep := range val.Chart.DependsOn {
					if !slices.Contains(addonNames, dep) {
						return nil, fmt.Errorf("addon %s depends on %s which is not present in the list of addons", val.Name, dep)
					}
				}
			}
		}

		if strings.EqualFold(kindManifest, val.Kind) {
			if val.Chart != nil {
				blueprintlog.Info("received chart object.", "Kind", kindManifest)
				return nil, fmt.Errorf("chart object is not allowed for addon kind %s", kindManifest)
			}
			if val.Manifest == nil {
				blueprintlog.Info("received empty manifest object.", "Kind", kindManifest)
				return nil, fmt.Errorf("manifest object can't be empty for addon kind %s", kindManifest)
			}
		}
	}

	return nil, nil
}
