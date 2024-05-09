package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fluxcd/helm-controller/api/v2beta2"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/source-controller/api/v1beta2"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/pkg/consts"
)

const (
	defaultHelmReleaseInterval = 5 * time.Minute
)

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

// CreateHelmRelease creates a HelmRelease object in the given namespace
func (hc *Controller) CreateHelmRelease(addon *v1alpha1.Addon, targetNamespace string, isDryRun bool) error {
	chartSpec := addon.Spec.Chart
	repo := v1beta2.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetRepoName(addon),
			Namespace: consts.NamespaceBoundlessSystem,
		},
		Spec: v1beta2.HelmRepositorySpec{
			URL: chartSpec.Repo,
			Interval: metav1.Duration{
				Duration: defaultHelmReleaseInterval,
			},
		},
	}

	var values *apiextensionsv1.JSON
	if chartSpec.Values != "" {
		v, _ := yaml.YAMLToJSON([]byte(chartSpec.Values))
		values = &apiextensionsv1.JSON{Raw: v}
	}

	release := v2beta2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chartSpec.Name,
			Namespace: consts.NamespaceBoundlessSystem,
		},
		Spec: v2beta2.HelmReleaseSpec{
			TargetNamespace: targetNamespace,
			ReleaseName:     chartSpec.Name,
			Chart: v2beta2.HelmChartTemplate{
				Spec: v2beta2.HelmChartTemplateSpec{
					Chart:   chartSpec.Name,
					Version: chartSpec.Version,
					SourceRef: v2beta2.CrossNamespaceObjectReference{
						Name: GetRepoName(addon),
						Kind: "HelmRepository",
					},
				},
			},
			DriftDetection: &v2beta2.DriftDetection{
				Mode: v2beta2.DriftDetectionEnabled,
			},
			Values: values,
			Interval: metav1.Duration{
				Duration: 10 * time.Second,
			},
		},
	}

	controllerutil.SetOwnerReference(addon, &release, hc.client.Scheme())

	if isDryRun {
		// TODO - Jira Ticket: https://mirantis.jira.com/browse/BOP-585
	}

	return hc.createOrUpdateHelmRelease(repo, release)
}

// DeleteHelmRelease deletes a HelmRelease object in the given namespace
func (hc *Controller) DeleteHelmRelease(addon *v1alpha1.Addon) error {
	release := v2beta2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.Spec.Chart.Name,
			Namespace: consts.NamespaceBoundlessSystem,
		},
	}

	// set a deadline for the Kubernetes API operations
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := hc.getExistingHelmRelease(ctx, release.Namespace, release.Name)
	if err != nil {
		return err
	}

	if existing == nil {
		hc.logger.Info("helm release to clean up does not exist", "HelmReleaseName", release.GetName())
		return nil
	}

	err = hc.client.Delete(ctx, &release)
	if err != nil {
		hc.logger.Error(err, "failed to delete helm chart", "HelmReleaseName", release.GetName())
		return err
	}

	hc.logger.Info("helm chart successfully deleted", "HelmReleaseName", release.GetName())
	return nil
}

func (hc *Controller) createOrUpdateHelmRelease(repo v1beta2.HelmRepository, release v2beta2.HelmRelease) error {
	if err := hc.createOrUpdateRepo(repo); err != nil {
		return fmt.Errorf("failed to create or update helm repository: %w", err)
	}

	if err := hc.createOrUpdateRelease(release); err != nil {
		return fmt.Errorf("failed to create or update helm release: %w", err)
	}

	return nil
}

func (hc *Controller) createOrUpdateRelease(release v2beta2.HelmRelease) error {
	// set a deadline for the Kubernetes API operations
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := hc.getExistingHelmRelease(ctx, release.Namespace, release.Name)
	if err != nil {
		return err
	}

	if existing != nil {

		hc.logger.Info("helm release already exists, updating", "HelmReleaseName", release.GetName())
		release.SetResourceVersion(existing.GetResourceVersion())
		err = hc.client.Update(ctx, &release)
		hc.logger.Info("helm release updated", "HelmReleaseName", release.GetName())
	} else {
		hc.logger.Info("helm release does not exists, creating", "HelmReleaseName", release.GetName(), "Namespace", release.GetNamespace())
		err = hc.client.Create(ctx, &release)
		if err != nil {
			return err
		}
		hc.logger.Info("helm release created", "ChartName", release.GetName())
	}

	return nil
}

func (hc *Controller) createOrUpdateRepo(repo v1beta2.HelmRepository) error {
	// set a deadline for the Kubernetes API operations
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	existing, err := hc.getExistingHelmRepo(ctx, repo.Namespace, repo.Name)
	if err != nil {
		return err
	}

	if existing != nil {
		hc.logger.Info("helm repository already exists, updating", "HelmRepositoryName", repo.GetName())
		repo.SetResourceVersion(existing.GetResourceVersion())
		err = hc.client.Update(ctx, &repo)
		hc.logger.Info("helm repository updated", "HelmRepositoryName", repo.GetName())
	} else {
		hc.logger.Info("helm repository does not exists, creating", "URL", repo.Spec.URL)
		err = hc.client.Create(ctx, &repo)
		if err != nil {
			return err
		}
		hc.logger.Info("helm repository created", "HelmRepositoryName", repo.GetName())
	}

	return nil

}

func (hc *Controller) getExistingHelmRelease(ctx context.Context, namespace, name string) (*v2beta2.HelmRelease, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existing := &v2beta2.HelmRelease{}
	err := hc.client.Get(ctx, key, existing)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			hc.logger.Info("helm release does not exist", "Namespace", namespace, "ReleaseName", name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to get existing helm release: %w", err)
		}
	}
	return existing, nil
}

func (hc *Controller) getExistingHelmRepo(ctx context.Context, namespace, name string) (*v1beta2.HelmRepository, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	existing := &v1beta2.HelmRepository{}
	err := hc.client.Get(ctx, key, existing)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			hc.logger.Info("helm repo does not exist", "Namespace", namespace, "HelmRepositoryName", name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to get existing helm repository: %w", err)
		}
	}
	return existing, nil
}

// GetRepoName returns the name of the HelmRepository object
func GetRepoName(addon *v1alpha1.Addon) string {
	return fmt.Sprintf("repo-%s-%s", addon.Name, addon.Spec.Chart.Name)
}
