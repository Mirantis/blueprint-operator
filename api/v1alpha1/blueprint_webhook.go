package v1alpha1

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

const (
	kindManifest = "manifest"
	kindChart    = "chart"
)

// log is for logging in this package.
var blueprintlog = logf.Log.WithName("blueprint-resource")

func (r *Blueprint) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-boundless-mirantis-com-v1alpha1-blueprint,mutating=true,failurePolicy=fail,sideEffects=None,groups=boundless.mirantis.com,resources=blueprints,verbs=create;update,versions=v1alpha1,name=mblueprint.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Blueprint{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Blueprint) Default() {
	blueprintlog.Info("default", "name", r.Name)
}

// change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-boundless-mirantis-com-v1alpha1-blueprint,mutating=false,failurePolicy=fail,sideEffects=None,groups=boundless.mirantis.com,resources=blueprints,verbs=create;update,versions=v1alpha1,name=vblueprint.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Blueprint{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Blueprint) ValidateCreate() (admission.Warnings, error) {
	blueprintlog.Info("validate create", "name", r.Name)
	return validate(r.Spec)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Blueprint) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	blueprintlog.Info("validate update", "name", r.Name)
	return validate(r.Spec)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Blueprint) ValidateDelete() (admission.Warnings, error) {
	blueprintlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

func validate(spec BlueprintSpec) (admission.Warnings, error) {
	if len(spec.Components.Addons) > 0 {
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
	}

	return nil, nil
}
