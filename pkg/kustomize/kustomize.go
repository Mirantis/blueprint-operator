package kustomize

import (
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/yaml"

	"github.com/mirantiscontainers/blueprint-operator/client/api/v1alpha1"
)

// Render uses the manifest url and values from the blueprint and generates kustomization.yaml.
// It also generates kustomize build output and returns it.
func Render(logger logr.Logger, url string, values *v1alpha1.Values) ([]byte, error) {
	fs := filesys.MakeFsInMemory()

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	var labels []kustypes.Label
	var resources []string
	var images []kustypes.Image
	var patches []kustypes.Patch

	// This shall add the following label to all manifest objects
	labels = append(labels, kustypes.Label{Pairs: map[string]string{"com.mirantis.blueprint/controlled-by": "blueprint"}, IncludeSelectors: true})
	resources = append(resources, url)

	if values != nil {
		for _, p := range values.Patches {
			patches = append(kus.Patches, kustypes.Patch{
				Path:    p.Path,
				Patch:   p.Patch,
				Options: p.Options,
				Target:  convertSelector(p.Target),
			})
		}

		for _, i := range values.Images {
			images = append(images, convertImage(i))
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

func convertSelector(target *v1alpha1.Selector) *kustypes.Selector {
	if target == nil {
		return nil
	}

	return &kustypes.Selector{
		ResId: resid.ResId{
			Gvk: resid.Gvk{
				Group:   target.Group,
				Version: target.Version,
				Kind:    target.Kind,
			},
			Namespace: target.Namespace,
			Name:      target.Name,
		},
		AnnotationSelector: target.AnnotationSelector,
		LabelSelector:      target.LabelSelector,
	}
}

func convertImage(image v1alpha1.Image) kustypes.Image {
	return kustypes.Image{
		Name:      image.Name,
		NewName:   image.NewName,
		TagSuffix: image.TagSuffix,
		NewTag:    image.NewTag,
		Digest:    image.Digest,
	}
}
