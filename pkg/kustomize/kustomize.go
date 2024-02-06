package kustomize

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"

	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"

	kustypes "sigs.k8s.io/kustomize/api/types"
)

const (
	dirPath = "/tmp/"
)

// GenerateKustomization uses the manifest url and values from the blueprint and generates kustomization.yaml.
// It also generates kustomize build output and returns it along with the name of the kustomization file.
func GenerateKustomization(logger logr.Logger, manifestSpec *boundlessv1alpha1.ManifestInfo) (string, string, error) {
	fs := filesys.MakeFsOnDisk()

	s, err := utils.RandDirName(10)
	if err != nil {
		logger.Error(err, "error generating random name", "Error", err)
		return "", "", err
	}

	if err := os.Mkdir(dirPath+s, os.ModePerm); err != nil {
		logger.Error(err, "failed to create directory", "DIR", dirPath+s)
		return "", "", err
	}

	// This function is temporary and will eventually be added in Manifest controller as part of BOP-277.
	defer func() {
		if err := os.RemoveAll(dirPath + s); err != nil {
			logger.Error(err, "failed to delete directory", "DIR", dirPath+s, "Error", err)
		}
	}()

	abs, err := filepath.Abs(dirPath + s)
	if err != nil {
		return "", "", err
	}

	kfile := filepath.Join(abs, konfig.DefaultKustomizationFileName())
	f, err := fs.Create(kfile)
	if err != nil {
		logger.Error(err, "error while creating file", "Error", err)
		return "", "", err
	}
	f.Close()

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	var resources []string
	var images []kustypes.Image
	var patches []kustypes.Patch

	resources = append(resources, manifestSpec.URL)

	if manifestSpec.Values != nil {
		if len(manifestSpec.Values.Images) > 0 {
			for i := range manifestSpec.Values.Images {
				image := kustypes.Image{
					Name:      manifestSpec.Values.Images[i].Name,
					NewName:   manifestSpec.Values.Images[i].NewName,
					TagSuffix: manifestSpec.Values.Images[i].TagSuffix,
					NewTag:    manifestSpec.Values.Images[i].NewTag,
					Digest:    manifestSpec.Values.Images[i].Digest,
				}
				images = append(images, image)
			}
		}

		if len(manifestSpec.Values.Patches) > 0 {
			for i := range manifestSpec.Values.Patches {
				patch := kustypes.Patch{
					Path:    manifestSpec.Values.Patches[i].Path,
					Patch:   manifestSpec.Values.Patches[i].Patch,
					Options: manifestSpec.Values.Patches[i].Options,
				}
				if manifestSpec.Values.Patches[i].Target != nil {
					target := &kustypes.Selector{
						ResId:              manifestSpec.Values.Patches[i].Target.ResId,
						AnnotationSelector: manifestSpec.Values.Patches[i].Target.AnnotationSelector,
						LabelSelector:      manifestSpec.Values.Patches[i].Target.LabelSelector,
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

	kd, err := yaml.Marshal(kus)
	if err != nil {
		return "", "", fmt.Errorf("%v", err)
	}

	err = os.WriteFile(kfile, kd, os.ModePerm)
	if err != nil {
		logger.Error(err, "error while writing file", "File", kfile, "Error", err)
		return "", "", fmt.Errorf("%v", err)
	}

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	m, err := k.Run(fs, abs)
	if err != nil {
		return "", "", err
	}

	objects, err := m.AsYaml()
	if err != nil {
		return "", "", err
	}

	return kfile, string(objects), nil

}
