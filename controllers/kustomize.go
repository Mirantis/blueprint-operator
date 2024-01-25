package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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

	s, err := RandDirName(10)
	if err != nil {
		logger.Info("Sakshi::ERROR generating random name", "Error", err)
		return nil, err
	}

	if err := os.Mkdir(dirPath+s, os.ModePerm); err != nil {
		logger.Info("Sakshi::Failed to create directory", "DIR", dirPath+s)
		return nil, err
	}

	defer func() {
		if err := os.RemoveAll(dirPath + s); err != nil {
			logger.Info("Sakshi::Failed to delete directory", "DIR", dirPath+s, "Error", err)
		}

		// Check if the directory is deleted
		files, err := os.ReadDir(dirPath + s)
		if err != nil {
			logger.Info("Sakshi::Failed to read directory", "DIR", dirPath+s, "Error", err)
		}
		for _, file := range files {
			logger.Info("Sakshi::Files in directory", "DIR", dirPath+s, "File", file.Name())
		}
	}()

	abs, err := filepath.Abs(dirPath + s)
	if err != nil {
		return nil, err
	}
	logger.Info("Sakshi::Absolute path name", "Path", abs)

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

func RandDirName(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:n], nil
}
