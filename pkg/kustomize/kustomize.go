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

// GenerateKustomization uses the manifest url and values from the blueprint and generates kustomization.yaml.
// It also generates kustomize build output and returns it.
func GenerateKustomization(logger logr.Logger, url string, patches []boundlessv1alpha1.Patch, images []boundlessv1alpha1.Image) ([]byte, error) {
	fs := filesys.MakeFsInMemory()

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	var resources []string
	var kusImages []kustypes.Image
	var kusPatches []kustypes.Patch

	resources = append(resources, url)

	if len(images) > 0 {
		for i := range images {
			image := kustypes.Image{
				Name:      images[i].Name,
				NewName:   images[i].NewName,
				TagSuffix: images[i].TagSuffix,
				NewTag:    images[i].NewTag,
				Digest:    images[i].Digest,
			}
			kusImages = append(kusImages, image)
		}
	}

	if len(patches) > 0 {
		for i := range patches {
			patch := kustypes.Patch{
				Path:    patches[i].Path,
				Patch:   patches[i].Patch,
				Options: patches[i].Options,
			}
			if patches[i].Target != nil {
				target := &kustypes.Selector{
					ResId:              patches[i].Target.ResId,
					AnnotationSelector: patches[i].Target.AnnotationSelector,
					LabelSelector:      patches[i].Target.LabelSelector,
				}
				patch.Target = target
			}
			kusPatches = append(kusPatches, patch)
		}
	}

	kus.Resources = resources
	kus.Patches = kusPatches
	kus.Images = kusImages

	kd, err := yaml.Marshal(kus)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	err = fs.WriteFile(konfig.DefaultKustomizationFileName(), kd)
	if err != nil {
		logger.Error(err, "error while writing file", "File", konfig.DefaultKustomizationFileName(), "Error", err)
		return nil, fmt.Errorf("%v", err)
	}

	logger.Info("kustomize file contents", "Contents", string(kd))

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
