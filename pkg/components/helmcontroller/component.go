package helmcontroller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/components"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
	"github.com/mirantiscontainers/boundless-operator/pkg/kubernetes"
	"github.com/mirantiscontainers/boundless-operator/pkg/utils"
)

const (
	deploymentHelmController = "helm-controller"
)

// CertManagerComponent is the manifest for installing cert manager.
type component struct {
	client client.Client
	logger logr.Logger
}

// NewHelmControllerComponent creates a new helm controller component.
func NewHelmControllerComponent(client client.Client, logger logr.Logger) components.Component {
	return &component{
		client: client,
		logger: logger,
	}
}

func (c *component) Name() string {
	return "helm-controller"
}

func (c *component) Install(ctx context.Context) error {
	var err error
	c.logger.Info("Installing helm controller")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(c.logger, c.client)
	if err := applier.Apply(ctx, kubernetes.NewManifestReader([]byte(HelmControllerTemplate))); err != nil {
		return err
	}

	// wait for helm controller to be ready
	c.logger.Info("Waiting for helm controller")
	if err = utils.WaitForDeploymentReady(ctx, c.logger, c.client, deploymentHelmController, consts.NamespaceBoundlessSystem); err != nil {
		return err
	}

	c.logger.Info("Finished installing helm controller")

	return nil
}

// Uninstall uninstalls cert manager from the cluster.
func (c *component) Uninstall(ctx context.Context) error {
	c.logger.Info("Uninstalling helm controller")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	applier := kubernetes.NewApplier(c.logger, c.client)
	reader := kubernetes.NewManifestReader([]byte(HelmControllerTemplate))
	objs, err := reader.ReadManifest()
	if err != nil {
		return err
	}

	if err := applier.Delete(ctx, objs); err != nil {
		return err
	}

	c.logger.Info("Finished uninstalling helm controller")
	return nil
}

// CheckExists checks if the helm controller exists in the cluster
func (c *component) CheckExists(ctx context.Context) (bool, error) {
	key := client.ObjectKey{
		Namespace: consts.NamespaceBoundlessSystem,
		Name:      deploymentHelmController,
	}
	if err := c.client.Get(ctx, key, &v1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
