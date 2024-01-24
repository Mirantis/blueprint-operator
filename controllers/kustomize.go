package controllers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	//"github.com/fluxcd/flux2/v2/pkg/manifestgen/kustomization"
	"github.com/go-logr/logr"

	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	kustypes "sigs.k8s.io/kustomize/api/types"
)

const (
	dirPath = "/tmp/"
)

func generateKustomization(logger logr.Logger, url string) ([]byte, error) {

	fs := filesys.MakeFsOnDisk()
	abs, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, err
	}
	kfile := filepath.Join(abs, konfig.DefaultKustomizationFileName())

	logger.Info("Sakshi::", "KFILE", kfile)
	f, err := fs.Create(kfile)

	if err != nil {
		logger.Info("Sakshi::ERROR while creating file", "Error", err)
		return nil, err
	}
	f.Close()

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	var resources []string
	resources = append(resources, url)

	kus.Resources = resources
	kd, err := yaml.Marshal(kus)

	logger.Info("Sakshi::FILE CONTENTS", "KD", string(kd))

	if err != nil {
		// delete the kustomization file
		return nil, fmt.Errorf("%v", err)
	}

	err = os.WriteFile(kfile, kd, os.ModePerm)
	if err != nil {
		logger.Info("Sakshi::ERROR while writing file", "Error", err)
		//TODO: delete the kustomization file
		return nil, fmt.Errorf("%v", err)
	}

	files, err := ioutil.ReadDir(abs)
	if err != nil {
		logger.Info("Sakshi::ERROR failed to read dir", "Error", err)
		return nil, fmt.Errorf("%v", err)
	}

	for _, file := range files {
		logger.Info("Sakshi: FILES", "FILENAME", file.Name())
	}

	// Kustomize Build
	/*objects, err := kustomization.BuildWithRoot(root, filepath.Dir(dirPath))
	if err != nil {
		logger.Info("Sakshi: Failed to build with Kustomize", "FILENAME", file.Name())
		return nil, fmt.Errorf("%v", err)
	}

	logger.Info("Sakshi: KUSTOMIZE OBJECTS", "Objects", objects)
	*/
	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	m, err := k.Run(fs, abs)
	if err != nil {
		return nil, err
	}

	objects, err := m.AsYaml()
	if err != nil {
		return nil, err
	}

	logger.Info("Sakshi: KUSTOMIZE OBJECTS", "Objects", string(objects))
	return objects, nil

}
