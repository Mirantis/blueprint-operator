package fluxcd

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/pkg/components"
)

type fluxcd struct {
	client client.Client
	logger logr.Logger
}

// NewFluxCDComponent creates a new instance of the fluxcd component.
func NewFluxCDComponent(client client.Client, logger logr.Logger) components.Component {
	return &fluxcd{
		client: client,
		logger: logger,
	}
}

func (f fluxcd) Name() string {
	return "fluxcd"
}

func (f fluxcd) Install(ctx context.Context) error {
	f.logger.Info("Installing fluxcd")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := installCRDs(f.client, f.logger); err != nil {
		return fmt.Errorf("failed to install fluxcd CRDs: %w", err)
	}

	if err := installComponents(f.client, f.logger); err != nil {
		return fmt.Errorf("failed to install fluxcd components: %w", err)
	}
	f.logger.Info("fluxcd installed successfully")
	return nil
}

func (f fluxcd) Uninstall(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (f fluxcd) CheckExists(ctx context.Context) (bool, error) {
	key := client.ObjectKey{
		Namespace: "flux-system",
		Name:      "helm-controller",
	}

	if err := f.client.Get(ctx, key, &v1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
