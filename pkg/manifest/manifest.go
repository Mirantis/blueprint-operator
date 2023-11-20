package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
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
	sum, err := mc.getCheckSumUrl(url)
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
			Url:      url,
			Checksum: sum,
		},
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
		// ToDo : add code for update
		// Use checksum to see if any updates are required.
		return nil

	} else {
		mc.logger.Info("manifest crd does not exist, creating", "ManifestName", m.Name, "Namespace", m.Namespace)

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

func (mc *ManifestController) getCheckSumUrl(url string) (string, error) {
	var Client http.Client

	// Run http get request to fetch the contents of the manifest file
	resp, err := Client.Get(url)
	if err != nil {
		mc.logger.Error(err, "failed to read response")
		return "", err
	}

	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			mc.logger.Error(err, "failed to read http response body")
			return "", err
		}

	} else {
		mc.logger.Error(err, "failure in http get request", "ResponseCode", resp.StatusCode)
		return "", err
	}

	sum := sha256.Sum256(bodyBytes)
	mc.logger.Info("computed checksum :", "Checksum", hex.EncodeToString(sum[:]))

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
