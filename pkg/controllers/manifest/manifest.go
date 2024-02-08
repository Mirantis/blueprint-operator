package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/kustomize"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ManifestController struct {
	client client.Client
	logger logr.Logger
}

func NewManifestController(client client.Client, logger logr.Logger) *ManifestController {
	return &ManifestController{
		client: client,
		logger: logger,
	}
}

func (mc *ManifestController) CreateManifest(namespace, name string, manifestSpec *boundlessv1alpha1.ManifestInfo) error {
	mc.logger.Info("Sakshi:: Received Values", "Patches", manifestSpec.Values.Patches, "Images", manifestSpec.Values.Images)

	var images []boundlessv1alpha1.Image
	var patches []boundlessv1alpha1.Patch

	if manifestSpec.Values != nil {
		images = manifestSpec.Values.Images
		patches = manifestSpec.Values.Patches
	}

	dataBytes, err := kustomize.GenerateKustomization(mc.logger, manifestSpec.URL, patches, images)
	if err != nil {
		mc.logger.Error(err, "failed to build kustomize for url: %s", "URL", manifestSpec.URL)
		return err
	}

	sum, err := mc.getCheckSumUrl(dataBytes)
	if err != nil {
		mc.logger.Error(err, "Failed to get checksum for url")
		return err
	}

	m := boundlessv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: boundlessv1alpha1.ManifestSpec{
			Url:      manifestSpec.URL,
			Checksum: sum,
		},
	}

	if manifestSpec.Values != nil {
		m.Spec.Patches = manifestSpec.Values.Patches
		m.Spec.Images = manifestSpec.Values.Images
	}

	return mc.createOrUpdateManifest(m)

}

func (mc *ManifestController) createOrUpdateManifest(m boundlessv1alpha1.Manifest) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := mc.getExistingManifest(m.Namespace, m.Name)
	if err != nil {
		return err
	}

	if existing != nil {
		// Use checksum to see if any updates are required.
		if existing.Spec.Checksum != m.Spec.Checksum {
			mc.logger.Info("manifest crd exists, checksum differs", "Existing", existing.Spec.Checksum, "New", m.Spec.Checksum)

			// This will differ both in case either the url has changed or the contents of the url has changed.
			// Store the newChecksum to the new computed value and store checksum to the old value.
			// This value will be reset by manifest controller after the update workflow is completed.

			newManifest := boundlessv1alpha1.Manifest{
				ObjectMeta: metav1.ObjectMeta{
					Name:            m.Name,
					Namespace:       m.Namespace,
					ResourceVersion: existing.ResourceVersion,
				},
				Spec: boundlessv1alpha1.ManifestSpec{
					Url:         m.Spec.Url,
					Checksum:    existing.Spec.Checksum,
					NewChecksum: m.Spec.Checksum,
					Objects:     existing.Spec.Objects,
					Patches:     m.Spec.Patches,
					Images:      m.Spec.Images,
				},
			}
			newManifest.SetFinalizers(existing.GetFinalizers())
			err := mc.client.Update(ctx, &newManifest)
			if err != nil {
				mc.logger.Info("failed to update manifest crd", "Error", err)
				return err
			}
			mc.logger.Info("manifest updated successfully", "ManifestName", m.Name)
		}
		return nil

	} else {
		mc.logger.Info("manifest crd does not exist, creating", "ManifestName", m.Name, "Namespace", m.Namespace)

		// In this case, NewChecksum will be an empty string
		//m.Spec.NewChecksum = m.Spec.Checksum
		m.Spec.NewChecksum = ""

		err := mc.client.Create(ctx, &m)
		if err != nil {
			mc.logger.Info("failed to create manifest crd", "Error", err)
			return err
		}
		mc.logger.Info("manifest created successfully", "ManifestName", m.Name)
	}

	return nil
}

func (mc *ManifestController) getExistingManifest(namespace, name string) (*boundlessv1alpha1.Manifest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existing := &boundlessv1alpha1.Manifest{}
	err := mc.client.Get(ctx, key, existing)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			mc.logger.Info("manifest does not exist", "Namespace", namespace, "ManifestName", name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to get existing manifest: %w", err)
		}
	}
	return existing, nil
}

func (mc *ManifestController) getCheckSumUrl(kustomizeBytes []byte) (string, error) {
	sum := sha256.Sum256(kustomizeBytes)
	mc.logger.Info("computed checksum on kustomize build output", "Checksum", hex.EncodeToString(sum[:]))
	return hex.EncodeToString(sum[:]), nil
}

func (mc *ManifestController) DeleteManifest(namespace, name, url string) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := mc.getExistingManifest(namespace, name)
	if err != nil {
		return err
	}

	if existing == nil {
		mc.logger.Info("manifest object does not exist", "Name", name)
		return nil

	}

	mc.logger.Info("deleting the manifest crd", "ManifestName", name, "Namespace", namespace)

	err = mc.client.Delete(ctx, existing)
	if err != nil {
		mc.logger.Info("failed to delete manifest crd", "Error", err)
		return err
	}
	mc.logger.Info("manifest deleted successfully", "ManifestName", name)

	return nil

}
