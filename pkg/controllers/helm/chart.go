package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

// this is the image that is used by helm controller to actually run the helm install
const helmJobImage = "ghcr.io/mirantiscontainers/klipper-helm:42274ad"

type Controller struct {
	client client.Client
	logger logr.Logger
}

func NewHelmChartController(client client.Client, logger logr.Logger) *Controller {
	return &Controller{
		client: client,
		logger: logger,
	}
}

// CreateHelmChart creates a HelmChart CRD in the given namespace
func (hc *Controller) CreateHelmChart(info *v1alpha1.ChartInfo, targetNamespace string, isDryRun bool) error {
	helmChart := helmv1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      info.Name,
			Namespace: targetNamespace,
		},
		Spec: helmv1.HelmChartSpec{
			TargetNamespace: targetNamespace,
			Chart:           info.Name,
			Version:         info.Version,
			Repo:            info.Repo,
			Set:             info.Set,
			ValuesContent:   info.Values,
			JobImage:        helmJobImage,
		},
	}

	if isDryRun {
		helmChart.Spec.DryRun = "server"
	}

	return hc.createOrUpdateHelmChart(helmChart)
}

// DeleteHelmChart deletes a HelmChart CRD in the given namespace
func (hc *Controller) DeleteHelmChart(info *v1alpha1.ChartInfo, targetNamespace string) error {

	chart := helmv1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      info.Name,
			Namespace: targetNamespace,
		},
		Spec: helmv1.HelmChartSpec{
			TargetNamespace: targetNamespace,
			Chart:           info.Name,
			Version:         info.Version,
			Repo:            info.Repo,
			Set:             info.Set,
			ValuesContent:   info.Values,
			JobImage:        helmJobImage,
		},
	}

	// set a deadline for the Kubernetes API operations
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := hc.getExistingHelmChart(ctx, chart.Namespace, chart.Name)
	if err != nil {
		return err
	}

	if existing == nil {
		hc.logger.Info("helm chart to clean up does not exist", "ChartName", chart.GetName())
		return nil
	}

	err = hc.client.Delete(ctx, &chart)
	if err != nil {
		hc.logger.Error(err, "failed to delete helm chart", "ChartName", chart.GetName())
		return err
	}

	hc.logger.Info("helm chart successfully deleted", "ChartName", chart.GetName())
	return nil
}

func (hc *Controller) createOrUpdateHelmChart(chart helmv1.HelmChart) error {
	// set a deadline for the Kubernetes API operations
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := hc.getExistingHelmChart(ctx, chart.Namespace, chart.Name)
	if err != nil {
		return err
	}

	if existing != nil {

		hc.logger.Info("helm chart already exists, updating", "ChartName", chart.GetName())
		chart.SetResourceVersion(existing.GetResourceVersion())
		err = hc.client.Update(ctx, &chart)
		hc.logger.Info("helm chart updated", "ChartName", chart.GetName())
	} else {
		hc.logger.Info("helm chart does not exists, creating", "ChartName", chart.GetName(), "Namespace", chart.GetNamespace())
		err = hc.client.Create(ctx, &chart)
		if err != nil {
			return err
		}
		hc.logger.Info("helm chart created", "ChartName", chart.GetName())
	}
	return nil
}

func (hc *Controller) getExistingHelmChart(ctx context.Context, namespace, name string) (*helmv1.HelmChart, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existing := &helmv1.HelmChart{}
	err := hc.client.Get(ctx, key, existing)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			hc.logger.Info("helm chart does not exist", "Namespace", namespace, "ChartName", name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to get existing helm chart: %w", err)
		}
	}
	return existing, nil
}
