package fluxcd

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/blueprint-operator/pkg/components"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	"github.com/mirantiscontainers/blueprint-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/blueprint-operator/pkg/manifest"
	"github.com/mirantiscontainers/blueprint-operator/pkg/utils"
)

var (
	//go:embed crds
	crdsFiles embed.FS

	//go:embed manifests
	manifestsFiles embed.FS
)

const (
	fluxCDNamespace    = "flux-system"
	helmControllerName = "helm-controller"

	// images

	// HelmControllerImageTag is the tag of the helm controller image
	HelmControllerImageTag = "v1.0.1"

	// KustomizeControllerImageTag is the tag of the kustomize controller image
	KustomizeControllerImageTag = "v1.3.0"

	// NotificationControllerImageTag is the tag of the notification controller image
	NotificationControllerImageTag = "v1.3.0"

	// SourceControllerImageTag is the tag of the source controller image
	SourceControllerImageTag = "v1.3.0"

	helmControllerImage         = "fluxcd/helm-controller:v1.0.1"
	kustomizeControllerImage    = "fluxcd/kustomize-controller:v1.3.0"
	notificationControllerImage = "fluxcd/notification-controller:v1.3.0"
	sourceControllerImage       = "fluxcd/source-controller:v1.3.0"
)

type fluxcdComponent struct {
	applier       *kubernetes.Applier
	client        client.Client
	logger        logr.Logger
	imageRegistry string
}

type imageConfig struct {
	HelmControllerImage         string
	KustomizeControllerImage    string
	NotificationControllerImage string
	SourceControllerImage       string
}

func newImageConfig(registry string) imageConfig {
	if registry == "" {
		registry = consts.MirantisImageRegistry
	}

	return imageConfig{
		HelmControllerImage:         fmt.Sprintf("%s/%s:%s", registry, helmControllerImage, HelmControllerImageTag),
		KustomizeControllerImage:    fmt.Sprintf("%s/%s:%s", registry, kustomizeControllerImage, KustomizeControllerImageTag),
		NotificationControllerImage: fmt.Sprintf("%s/%s:%s", registry, notificationControllerImage, NotificationControllerImageTag),
		SourceControllerImage:       fmt.Sprintf("%s/%s:%s", registry, sourceControllerImage, SourceControllerImageTag),
	}
}

// NewFluxCDComponent creates a new instance of the fluxcd component.
func NewFluxCDComponent(client client.Client, logger logr.Logger, imageRegistry string) components.Component {
	return &fluxcdComponent{
		applier:       kubernetes.NewApplier(logger, client),
		client:        client,
		logger:        logger,
		imageRegistry: imageRegistry,
	}
}

// Name returns the name of the component
func (c *fluxcdComponent) Name() string {
	return "fluxcd"
}

// Images returns the images used by fluxcd
func (c *fluxcdComponent) Images() []string {
	images := newImageConfig(c.imageRegistry)

	return []string{
		images.HelmControllerImage,
		images.KustomizeControllerImage,
		images.NotificationControllerImage,
		images.SourceControllerImage,
	}
}

// Install installs the fluxcd component
func (c *fluxcdComponent) Install(ctx context.Context) error {
	c.logger.Info("Installing fluxcd")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// create namespace if not exists
	if err := utils.CreateNamespaceIfNotExist(c.client, ctx, c.logger, fluxCDNamespace); err != nil {
		return fmt.Errorf("failed to create namespace flux-system: %w", err)
	}

	if err := c.installCRDs(ctx); err != nil {
		return fmt.Errorf("failed to install fluxcd CRDs: %w", err)
	}

	if err := c.installFluxCD(ctx); err != nil {
		return fmt.Errorf("failed to install fluxcd components: %w", err)
	}

	c.logger.Info("fluxcd installed successfully")
	return nil
}

// Uninstall uninstalls the fluxcd component
func (c *fluxcdComponent) Uninstall(ctx context.Context) error {
	c.logger.Info("Uninstalling fluxcd")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resources, err := manifest.Read(manifestsFiles, "manifests")
	if err != nil {
		return fmt.Errorf("failed to read FluxCD manifests: %w", err)
	}

	c.logger.Info("Deleting FluxCD resources")
	err = c.applier.Delete(ctx, resources)
	if err != nil {
		return fmt.Errorf("failed to delete FluxCD manifests: %w", err)
	}

	// Delete CRDs
	resources, err = manifest.Read(crdsFiles, "crds")
	if err != nil {
		return fmt.Errorf("failed to read FluxCD CRDs: %w", err)
	}

	if err = c.applier.Delete(ctx, resources); err != nil {
		return fmt.Errorf("failed to delete FluxCD CRDs: %w", err)
	}

	c.logger.Info("fluxcd uninstalled successfully")
	return nil
}

// CheckExists checks if the fluxcd component exists
func (c *fluxcdComponent) CheckExists(ctx context.Context) (bool, error) {
	key := client.ObjectKey{Namespace: fluxCDNamespace, Name: helmControllerName}

	if err := c.client.Get(ctx, key, &v1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (c *fluxcdComponent) installCRDs(ctx context.Context) error {
	c.logger.V(1).Info("Reading FluxCD CRDs")
	resources, err := manifest.Read(crdsFiles, "crds")
	if err != nil {
		return err
	}

	if err := c.applier.ApplyObjects(ctx, resources); err != nil {
		return fmt.Errorf("failed to apply FluxCD CRDs: %w", err)
	}

	var names []string
	for _, res := range resources {
		names = append(names, res.GetName())
	}

	// wait for CRDs to be created
	if err = components.WaitForCRDs(ctx, c.client, c.logger, names); err != nil {
		return err
	}

	return nil
}

func (c *fluxcdComponent) installFluxCD(ctx context.Context) error {
	images := newImageConfig(c.imageRegistry)

	resources, err := manifest.ReadTemplate(manifestsFiles, "manifests", images)
	if err != nil {
		return fmt.Errorf("failed to read FluxCD manifests: %w", err)
	}

	c.logger.Info("Applying FluxCD resources")

	err = c.applier.ApplyObjects(ctx, resources)
	if err != nil {
		return fmt.Errorf("failed to apply FluxCD manifests: %w", err)
	}

	return nil
}
