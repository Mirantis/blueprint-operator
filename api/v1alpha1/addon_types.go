package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/kyaml/resid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AddonSpec defines the desired state of Addon
type AddonSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Enum=manifest;chart;Manifest;Chart
	Kind string `json:"kind"`

	Enabled   bool          `json:"enabled"`
	Namespace string        `json:"namespace,omitempty"`
	Chart     *ChartInfo    `json:"chart,omitempty"`
	Manifest  *ManifestInfo `json:"manifest,omitempty"`
}

type ChartInfo struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Repo string `json:"repo"`

	// +kubebuilder:validation:Required
	Version string `json:"version"`

	Set    map[string]intstr.IntOrString `json:"set,omitempty"`
	Values string                        `json:"values,omitempty"`
}

type ManifestInfo struct {
	// +kubebuilder:validation:MinLength:=1
	URL    string  `json:"url"`
	Values *Values `json:"values,omitempty"`

	// This flag tells the controller how to handle the manifest in case of a failure.
	// Valid values are:
	// - None (default) : No-op; No action is triggered on manifest failure
	// - Retry : Manifest is retried in case of failure. For install, the manifest resources are deleted and re-installed.
	//			 For update, the new version of the manifest is applied on top of existing resources.
	// +optional
	FailurePolicy string `json:"failurePolicy,omitempty"`

	// Timeout for manifest operations as duration string (300s, 10m, 1h, etc)
	// If manifest is not Available after timeout duration, it will be handled by specified FailurePolicy
	// +optional
	Timeout string `json:"timeout,omitempty"`
}

type Values struct {
	// Patches is a list of patches, where each one can be either a
	// Strategic Merge Patch or a JSON patch.
	// Each patch can be applied to multiple target objects.
	Patches []Patch `json:"patches,omitempty"`

	// Images is a list of (image name, new name, new tag or digest)
	// for changing image names, tags or digests. This can also be achieved with a
	// patch, but this operator is simpler to specify.
	Images []Image `json:"images,omitempty"`
}

// Patch contains an inline StrategicMerge or JSON6902 patch, and the target the patch should
// be applied to. This is in coherence with https://github.com/kubernetes-sigs/kustomize/blob/api/v0.16.0/api/types/patch.go#L12
type Patch struct {
	// Path is a relative file path to the patch file.
	// +optional
	Path string `json:"path,omitempty"`

	// Patch contains an inline StrategicMerge patch or an inline JSON6902 patch with
	// an array of operation objects.
	// +required
	Patch string `json:"patch"`

	// Target points to the resources that the patch document should be applied to.
	// +optional
	Target *Selector `json:"target,omitempty"`

	// Options is a list of options for the patch
	// +optional
	Options map[string]bool `json:"options,omitempty"`
}

// Selector specifies a set of resources. Any resource that matches intersection of all conditions is included in this
// set.
type Selector struct {
	// ResId refers to a GVKN/Ns of a resource.
	resid.ResId `json:",inline,omitempty" yaml:",inline,omitempty"`

	// AnnotationSelector is a string that follows the label selection expression
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	// It matches with the resource annotations.
	AnnotationSelector string `json:"annotationSelector,omitempty" yaml:"annotationSelector,omitempty"`

	// LabelSelector is a string that follows the label selection expression
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	// It matches with the resource labels.
	LabelSelector string `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
}

// Image contains an image name, a new name, a new tag or digest, which will replace the original name and tag.
type Image struct {
	// Name is a tag-less image name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// NewName is the value used to replace the original name.
	NewName string `json:"newName,omitempty" yaml:"newName,omitempty"`

	// TagSuffix is the value used to suffix the original tag
	// If Digest and NewTag is present an error is thrown
	TagSuffix string `json:"tagSuffix,omitempty" yaml:"tagSuffix,omitempty"`

	// NewTag is the value used to replace the original tag.
	NewTag string `json:"newTag,omitempty" yaml:"newTag,omitempty"`

	// Digest is the value used to replace the original image tag.
	// If digest is present NewTag value is ignored.
	Digest string `json:"digest,omitempty" yaml:"digest,omitempty"`
}

// StatusType is a type of condition that may apply to a particular component.
type StatusType string

const (
	// TypeComponentAvailable indicates that the component is healthy.
	TypeComponentAvailable StatusType = "Available"

	// TypeComponentProgressing means that the component is in the process of being installed or upgraded.
	TypeComponentProgressing StatusType = "Progressing"

	// TypeComponentDegraded means the component is not operating as desired and user action is required.
	TypeComponentDegraded StatusType = "Degraded"

	// TypeComponentReady indicates that the component is healthy and ready.it is identical to Available.
	TypeComponentReady StatusType = "Ready"

	// TypeComponentUnhealthy indicates the component is not functioning as intended.
	TypeComponentUnhealthy StatusType = "Unhealthy"
)

type Status struct {
	// The type of condition. May be Available, Progressing, or Degraded.
	Type StatusType `json:"type"`

	// The timestamp representing the start time for the current status.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// A brief reason explaining the condition.
	Reason string `json:"reason,omitempty"`

	// Optionally, a detailed message providing additional context.
	Message string `json:"message,omitempty"`
}

// AddonStatus defines the observed state of Addon
type AddonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.type",description="Whether the component is running and stable."

// Addon is the Schema for the addons API
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AddonList contains a list of Addon
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}
