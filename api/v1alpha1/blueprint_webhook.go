package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var blueprintlog = logf.Log.WithName("blueprint-resource")

func (r *Blueprint) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-boundless-mirantis-com-v1alpha1-blueprint,mutating=true,failurePolicy=fail,sideEffects=None,groups=boundless.mirantis.com,resources=blueprints,verbs=create;update,versions=v1alpha1,name=mblueprint.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Blueprint{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Blueprint) Default() {
	blueprintlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-boundless-mirantis-com-v1alpha1-blueprint,mutating=false,failurePolicy=fail,sideEffects=None,groups=boundless.mirantis.com,resources=blueprints,verbs=create;update,versions=v1alpha1,name=vblueprint.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Blueprint{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Blueprint) ValidateCreate() (admission.Warnings, error) {
	blueprintlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Blueprint) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	blueprintlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Blueprint) ValidateDelete() (admission.Warnings, error) {
	blueprintlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
