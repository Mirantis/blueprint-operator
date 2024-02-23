package kustomize

import (
	"fmt"
	"github.com/go-logr/logr"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	kustypes "sigs.k8s.io/kustomize/api/types"
)

// Render uses the manifest url and values from the blueprint and generates kustomization.yaml.
// It also generates kustomize build output and returns it.
func Render(logger logr.Logger, url string, values *boundlessv1alpha1.Values) ([]byte, error) {
	fs := filesys.MakeFsInMemory()

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	var resources []string
	var images []kustypes.Image
	var patches []kustypes.Patch
	var labels []kustypes.Label

	// This shall add the following label to all manifest objects
	label := kustypes.Label{
		Pairs: map[string]string{
			"controlled-by": "com.mirantis.boundless",
		},
		IncludeSelectors: true,
	}

	labels = append(labels, label)

	resources = append(resources, url)

	if values != nil {
		if len(values.Images) > 0 {
			for i := range values.Images {
				image := kustypes.Image{
					Name:      values.Images[i].Name,
					NewName:   values.Images[i].NewName,
					TagSuffix: values.Images[i].TagSuffix,
					NewTag:    values.Images[i].NewTag,
					Digest:    values.Images[i].Digest,
				}
				images = append(images, image)
			}
		}

		if len(values.Patches) > 0 {
			for i := range values.Patches {
				patch := kustypes.Patch{
					Path:    values.Patches[i].Path,
					Patch:   values.Patches[i].Patch,
					Options: values.Patches[i].Options,
				}
				if values.Patches[i].Target != nil {
					target := &kustypes.Selector{
						ResId:              values.Patches[i].Target.ResId,
						AnnotationSelector: values.Patches[i].Target.AnnotationSelector,
						LabelSelector:      values.Patches[i].Target.LabelSelector,
					}
					patch.Target = target
				}
				patches = append(patches, patch)
			}
		}
	}

	kus.Resources = resources
	kus.Patches = patches
	kus.Images = images
	kus.Labels = labels

	kd, err := yaml.Marshal(kus)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	err = fs.WriteFile(konfig.DefaultKustomizationFileName(), kd)
	if err != nil {
		logger.Error(err, "error while writing file", "File", konfig.DefaultKustomizationFileName(), "Error", err)
		return nil, fmt.Errorf("%v", err)
	}

	logger.V(1).Info("kustomize file contents", "Contents", string(kd))

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	m, err := k.Run(fs, ".")
	if err != nil {
		return nil, err
	}

	objects, err := m.AsYaml()
	if err != nil {
		return nil, err
	}

	return objects, nil

}
