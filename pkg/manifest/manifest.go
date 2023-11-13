package manifest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	boundlessv1alpha1 "github.com/mirantis/boundless-operator/api/v1alpha1"

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

func (mc *ManifestController) CreateManifest(namespace, name, url string) error {
	/*manifest := boundlessv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: boundlessv1alpha1.ManifestSpec{
			Url: url,
		},
	}*/

	return mc.createOrUpdateManifest(namespace, name, url)

}

func (mc *ManifestController) createOrUpdateManifest(namespace, name, url string) error {

	mc.logger.Info("Sakshi:::::Fn createOrUpdateManifest() URL", "URL", url)
	m := boundlessv1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: boundlessv1alpha1.ManifestSpec{
			Url: url,
		},
	}

	mc.logger.Info("Sakshi:::::Fn createOrUpdateManifest() Manifest", "Manifest", m)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := mc.getExistingManifest(m.Namespace, m.Name)
	if err != nil {
		return err
	}

	if existing != nil {
		// ToDo : add code for update
		return nil

	} else {
		mc.logger.Info("manifest crd does not exist, creating", "ManifestName", m.Name, "Namespace", m.Namespace)

		err := mc.client.Create(ctx, &m)
		if err != nil {
			mc.logger.Info("failed to create manifest crd", "Error", err)
			return err
		}
		mc.logger.Info("Sakshi:: manifest created successfully", "Manifest", m)
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
