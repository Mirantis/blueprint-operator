package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/go-logr/logr"
	"github.com/mirantiscontainers/blueprint-operator/api/v1alpha1"
	"github.com/mirantiscontainers/blueprint-operator/pkg/consts"
	k8s "github.com/mirantiscontainers/blueprint-operator/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	helmRepoInterval       = 5 * time.Minute
	driftDetectionInterval = 30 * time.Second

	installationRetries = 3
	upgradeRetries      = 3
)

var (
	upgradeFailureStrategyRollback = helmv2.RollbackRemediationStrategy

	helmReleaseTypeMeta = metav1.TypeMeta{
		APIVersion: "helm.toolkit.fluxcd.io/v2",
		Kind:       "HelmRelease",
	}

	helmRepositoryTypeMeta = metav1.TypeMeta{
		APIVersion: "source.toolkit.fluxcd.io/v1",
		Kind:       "HelmRepository",
	}
)

type Controller struct {
	k8sClient *k8s.Client
	client    client.Client
	logger    logr.Logger
}

func NewHelmChartController(client client.Client, k8sClient *k8s.Client, logger logr.Logger) *Controller {
	return &Controller{
		client:    client,
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// CreateHelmRelease creates a HelmRelease object in the given namespace
func (hc *Controller) CreateHelmRelease(ctx context.Context, addon *v1alpha1.Addon, targetNamespace string, isDryRun bool) error {
	repoName := getRepoName(addon)
	releaseName := addon.Spec.Name
	chartSpec := addon.Spec.Chart

	helmRepoType := sourcev1.HelmRepositoryTypeDefault
	if strings.HasPrefix(chartSpec.Repo, "oci://") {
		helmRepoType = sourcev1.HelmRepositoryTypeOCI
	}

	repo := &sourcev1.HelmRepository{
		TypeMeta: helmRepositoryTypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      repoName,
			Namespace: consts.NamespaceBlueprintSystem,
		},
		Spec: sourcev1.HelmRepositorySpec{
			URL:  chartSpec.Repo,
			Type: helmRepoType,
			Interval: metav1.Duration{
				Duration: helmRepoInterval,
			},
		},
	}

	//var values *apiextensionsv1.JSON
	//if chartSpec.Values != "" {
	//	v, _ := yaml.YAMLToJSON([]byte(chartSpec.Values))
	//	values = &apiextensionsv1.JSON{Raw: v}
	//}

	var dependsOn []meta.NamespacedObjectReference
	for _, addonName := range chartSpec.DependsOn {
		dependsOn = append(dependsOn, meta.NamespacedObjectReference{
			Name:      addonName,
			Namespace: consts.NamespaceBlueprintSystem,
		})
	}

	release := &helmv2.HelmRelease{
		TypeMeta: helmReleaseTypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: consts.NamespaceBlueprintSystem,
		},
		Spec: helmv2.HelmReleaseSpec{
			TargetNamespace: targetNamespace,
			ReleaseName:     releaseName,
			Chart: &helmv2.HelmChartTemplate{
				Spec: helmv2.HelmChartTemplateSpec{
					Chart:   chartSpec.Name,
					Version: chartSpec.Version,
					SourceRef: helmv2.CrossNamespaceObjectReference{
						Name: repoName,
						Kind: "HelmRepository",
					},
					ReconcileStrategy: "Revision",
				},
			},
			Install: &helmv2.Install{
				DisableWait:     true,
				CreateNamespace: true,
				Remediation: &helmv2.InstallRemediation{
					Retries: installationRetries,
				},
			},
			Upgrade: &helmv2.Upgrade{
				DisableWait:   true,
				CleanupOnFail: true,
				Remediation: &helmv2.UpgradeRemediation{
					Retries:  upgradeRetries,
					Strategy: &upgradeFailureStrategyRollback,
				},
			},
			DriftDetection: &helmv2.DriftDetection{
				Mode: helmv2.DriftDetectionEnabled,
			},
			Values: chartSpec.Values,
			Interval: metav1.Duration{
				Duration: driftDetectionInterval,
			},
			DependsOn: dependsOn,
		},
	}

	// set owner reference
	if err := controllerutil.SetControllerReference(addon, release, hc.client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference for addon %q: %w", addon.Name, err)
	}

	if isDryRun {
		// TODO - Jira Ticket: https://mirantis.jira.com/browse/BOP-585
	}

	return hc.applyHelmRelease(ctx, repo, release)
}

// DeleteHelmRelease deletes a HelmRelease object in the given namespace
func (hc *Controller) DeleteHelmRelease(ctx context.Context, addon *v1alpha1.Addon) error {
	release := &helmv2.HelmRelease{
		TypeMeta: helmReleaseTypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.Spec.Chart.Name,
			Namespace: consts.NamespaceBlueprintSystem,
		},
	}

	repo := &sourcev1.HelmRepository{
		TypeMeta: helmRepositoryTypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      getRepoName(addon),
			Namespace: consts.NamespaceBlueprintSystem,
		},
	}

	if err := hc.k8sClient.Delete(ctx, release); err != nil {
		return fmt.Errorf("failed to delete helm release: %w", err)
	}

	if err := hc.k8sClient.Delete(ctx, repo); err != nil {
		return fmt.Errorf("failed to delete helm repository: %w", err)
	}

	return nil
}

func (hc *Controller) applyHelmRelease(ctx context.Context, repo *sourcev1.HelmRepository, release *helmv2.HelmRelease) error {

	hc.logger.Info("Applying helm repo", "HelmRepo", release.GetName())
	if err := hc.k8sClient.Apply(ctx, repo); err != nil {
		return fmt.Errorf("failed to create or update helm repository: %w", err)
	}

	hc.logger.Info("Applying helm release", "HelmReleaseName", release.GetName())
	if err := hc.k8sClient.Apply(ctx, release); err != nil {
		return fmt.Errorf("failed to create or update helm release: %w", err)
	}

	return nil
}

// getRepoName returns the name of the HelmRepository object
func getRepoName(addon *v1alpha1.Addon) string {
	return fmt.Sprintf("repo-%s-%s", addon.Name, addon.Spec.Chart.Name)
}
