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
	kustypes "sigs.k8s.io/kustomize/api/types"
)

const (
	dirPath = "/tmp/"
)

// GenerateKustomization uses the manifest url and values from the blueprint and generates kustomization.yaml.
// It also generates kustomize build output and returns it along with the name of the kustomization file.
func GenerateKustomization(logger logr.Logger, manifestSpec *boundlessv1alpha1.ManifestInfo) (string, []byte, error) {
	fs := filesys.MakeFsOnDisk()

	tempdir, err := os.MkdirTemp(dirPath, "addon-")
	if err != nil {
		logger.Error(err, "error generating temporary directory", "Error", err)
		return "", nil, err
	}

	logger.Info("Sakshi:: new temporary directory", "DIR", tempdir)

	abs, err := filepath.Abs(tempdir)
	if err != nil {
		return "", nil, err
	}

	kfile := filepath.Join(abs, konfig.DefaultKustomizationFileName())
	f, err := fs.Create(kfile)
	if err != nil {
		logger.Error(err, "error while creating file", "Error", err)
		return "", nil, err
	}
	f.Close()

	logger.Info("Sakshi:::The name of the kustomization file", "FileName", kfile)

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
		return "", nil, fmt.Errorf("%v", err)
	}

	err = os.WriteFile(kfile, kd, os.ModePerm)
	if err != nil {
		logger.Error(err, "error while writing file", "File", kfile, "Error", err)
		return "", nil, fmt.Errorf("%v", err)
	}

	logger.Info("Sakshi:::kustomize file contents", "KustomizeFile", string(kd))

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	m, err := k.Run(fs, abs)
	if err != nil {
		return "", nil, err
	}

	objects, err := m.AsYaml()
	if err != nil {
		return "", nil, err
	}

	return abs, objects, nil

}
