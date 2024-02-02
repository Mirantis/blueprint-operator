package kustomize

import (
	"crypto/rand"
	"encoding/base64"
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

func GenerateKustomization(logger logr.Logger, manifestSpec *boundlessv1alpha1.ManifestInfo) (string, error) {
	fs := filesys.MakeFsOnDisk()

	s, err := RandDirName(10)
	if err != nil {
		logger.Error(err, "error generating random name", "Error", err)
		return "", err
	}

	if err := os.Mkdir(dirPath+s, os.ModePerm); err != nil {
		logger.Error(err, "failed to create directory", "DIR", dirPath+s)
		return "", err
	}

	// This function is temporary and will eventually be added in Manifest controller as part of BOP-277.
	defer func() {
		if err := os.RemoveAll(dirPath + s); err != nil {
			logger.Error(err, "failed to delete directory", "DIR", dirPath+s, "Error", err)
		}
	}()

	abs, err := filepath.Abs(dirPath + s)
	if err != nil {
		return "", err
	}

	kfile := filepath.Join(abs, konfig.DefaultKustomizationFileName())
	//logger.Info("The name of the kustomization file", "FileName", kfile)
	f, err := fs.Create(kfile)
	if err != nil {
		logger.Error(err, "error while creating file", "Error", err)
		return "", err
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

	if manifestSpec.Config != nil {
		if len(manifestSpec.Config.Images) > 0 {
			for i := range manifestSpec.Config.Images {
				image := kustypes.Image{
					Name:      manifestSpec.Config.Images[i].Name,
					NewName:   manifestSpec.Config.Images[i].NewName,
					TagSuffix: manifestSpec.Config.Images[i].TagSuffix,
					NewTag:    manifestSpec.Config.Images[i].NewTag,
					Digest:    manifestSpec.Config.Images[i].Digest,
				}
				images = append(images, image)
			}
		}

		if len(manifestSpec.Config.Patches) > 0 {
			for i := range manifestSpec.Config.Patches {
				patch := kustypes.Patch{
					Path:    manifestSpec.Config.Patches[i].Path,
					Patch:   manifestSpec.Config.Patches[i].Patch,
					Options: manifestSpec.Config.Patches[i].Options,
				}
				if manifestSpec.Config.Patches[i].Target != nil {
					target := &kustypes.Selector{
						ResId:              manifestSpec.Config.Patches[i].Target.ResId,
						AnnotationSelector: manifestSpec.Config.Patches[i].Target.AnnotationSelector,
						LabelSelector:      manifestSpec.Config.Patches[i].Target.LabelSelector,
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
		return "", fmt.Errorf("%v", err)
	}

	//logger.Info("kustomize file contents", "KustomizeFile", string(kd))

	err = os.WriteFile(kfile, kd, os.ModePerm)
	if err != nil {
		logger.Error(err, "error while writing file", "File", kfile, "Error", err)
		//TODO: delete the directory
		return "", fmt.Errorf("%v", err)
	}

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	m, err := k.Run(fs, abs)
	if err != nil {
		return "", err
	}

	objects, err := m.AsYaml()
	if err != nil {
		return "", err
	}

	logger.Info("kustomize build objects", "Objects", string(objects))
	return kfile, nil

}

func RandDirName(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:n], nil
}
