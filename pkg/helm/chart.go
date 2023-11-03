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
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Chart struct {
	Name    string                        `yaml:"name"`
	Repo    string                        `yaml:"repo"`
	Version string                        `yaml:"version"`
	Set     map[string]intstr.IntOrString `yaml:"set,omitempty"`
	Values  string                        `yaml:"values,omitempty"`
}

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

func (hc *Controller) CreateHelmChart(chartSpec Chart, namespace string) error {

	helmChart := helmv1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chartSpec.Name,
			Namespace: namespace,
		},
		Spec: helmv1.HelmChartSpec{
			TargetNamespace: namespace,
			Chart:           chartSpec.Name,
			Version:         chartSpec.Version,
			Repo:            chartSpec.Repo,
			Set:             chartSpec.Set,
			ValuesContent:   chartSpec.Values,
		},
	}

	return hc.createOrUpdateHelmChart(helmChart)
}

func (hc *Controller) DeleteHelmChart(chartSpec Chart, namespace string) error {

	chart := helmv1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chartSpec.Name,
			Namespace: namespace,
		},
		Spec: helmv1.HelmChartSpec{
			TargetNamespace: namespace,
			Chart:           chartSpec.Name,
			Version:         chartSpec.Version,
			Repo:            chartSpec.Repo,
			Set:             chartSpec.Set,
			ValuesContent:   chartSpec.Values,
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
