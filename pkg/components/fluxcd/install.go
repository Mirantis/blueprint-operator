package fluxcd

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"path"

	"github.com/go-logr/logr"
	mfc "github.com/manifestival/controller-runtime-client"
	"github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
)

var (
	//go:embed crds
	crdsFiles embed.FS

	//go:embed manifests
	manifestsFiles embed.FS
)

func installCRDs(client client.Client, logger logr.Logger) error {
	objs, err := getObjs(crdsFiles, "crds")
	if err != nil {
		return fmt.Errorf("failed to get FluxCD CRDs: %w", err)
	}
	err = applyObjects(client, logger, objs)
	if err != nil {
		return fmt.Errorf("failed to apply fluxcd CRDs: %w", err)
	}
	return nil
}

func installComponents(client client.Client, logger logr.Logger) error {

	err := utils.CreateNamespaceIfNotExist(client, context.TODO(), logger, "flux-system")
	if err != nil {
		return fmt.Errorf("failed to create namespace flux-system: %w", err)
	}

	objs, err := getObjs(manifestsFiles, "manifests")
	if err != nil {
		return fmt.Errorf("failed to get FluxCD manifests: %w", err)
	}

	logger.Info("Applying FluxCD manifests")
	for _, obj := range objs {
		logger.Info(fmt.Sprintf("Applying %s/%s", obj.GetKind(), obj.GetName()))
	}

	err = applyObjects(client, logger, objs)
	if err != nil {
		return fmt.Errorf("failed to apply fluxcd manifests: %w", err)
	}

	return nil
}

func applyObjects(client client.Client, logger logr.Logger, objs []unstructured.Unstructured) error {
	mc := mfc.NewClient(client)
	opts := []manifestival.Option{
		manifestival.UseClient(mc),
		manifestival.UseLogger(logger),
	}
	manifests, err := manifestival.ManifestFrom(manifestival.Slice(objs), opts...)
	if err != nil {
		return err
	}

	err = manifests.Apply()
	if err != nil {
		return fmt.Errorf("failed to apply fluxcd objects: %w", err)
	}

	return nil

}

func getObjs(fs embed.FS, dir string) ([]unstructured.Unstructured, error) {
	var objs []unstructured.Unstructured

	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read FluxCD CRDs: %v", err)
	}

	for _, entry := range entries {
		b, err := fs.ReadFile(path.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read FluxCD CRD %s: %v", entry.Name(), err)
		}
		m, err := manifestival.ManifestFrom(manifestival.Reader(bytes.NewReader(b)))
		if err != nil {
			return nil, err
		}

		objs = append(objs, m.Resources()...)
	}

	return objs, nil

}
