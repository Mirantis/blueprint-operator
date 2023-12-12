package installation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextenv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/mirantis/boundless-operator/pkg/controllers/installation/manifests"
)

func InstallHelmController(ctx context.Context, runtimeClient client.Client, logger logr.Logger) error {
	var err error

	logger.Info("installing helm controller")

	// create namespace
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "boundless-system",
		},
	}

	if err = runtimeClient.Create(ctx, &ns); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create namespace: %v", err)
		}
	}

	svcAcc, err := yamlToServiceAccount([]byte(manifests.HelmServiceAccount))
	if err != nil {
		return fmt.Errorf("failed to parse service account: %v", err)
	}

	// create service account
	if err = runtimeClient.Create(ctx, svcAcc); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create service account: %v", err)
		}
	}
	// create crds
	crds1, err := yamlToCRD([]byte(manifests.HelmChart))
	if err != nil {
		return fmt.Errorf("failed to parse crd: %v", err)
	}

	crds2, err := yamlToCRD([]byte(manifests.HelmChartConig))
	if err != nil {
		return fmt.Errorf("failed to parse crd: %v", err)
	}

	crds3, err := yamlToCRD([]byte(manifests.HelmChartrelease))
	if err != nil {
		return fmt.Errorf("failed to parse crd: %v", err)
	}

	for _, crd := range []*apiextenv1.CustomResourceDefinition{crds1, crds2, crds3} {
		if err = runtimeClient.Create(ctx, crd); err != nil {
			if client.IgnoreAlreadyExists(err) != nil {
				return fmt.Errorf("failed to create crd: %v", err)
			}
		}
	}
	// create rbac
	role, err := yamlToClusterRole([]byte(manifests.ClusterRole))
	if err != nil {
		return fmt.Errorf("failed to parse cluster role: %v", err)
	}

	roleBinding1, err := yamlToClusterRoleBinding([]byte(manifests.HelmClusterRoleBinding1))
	if err != nil {
		return fmt.Errorf("failed to parse cluster role binding: %v", err)
	}

	roleBinding2, err := yamlToClusterRoleBinding([]byte(manifests.HelmClusterRoleBinding2))
	if err != nil {
		return fmt.Errorf("failed to parse cluster role binding: %v", err)
	}

	if err = runtimeClient.Create(ctx, role); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create cluster role: %v", err)
		}
	}

	if err = runtimeClient.Create(ctx, roleBinding1); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create clusterrole binding: %v", err)
		}
	}

	if err = runtimeClient.Create(ctx, roleBinding2); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create cluster role binding: %v", err)
		}
	}

	// create deployment
	deploy, err := yamlToDeployment([]byte(manifests.HelmDeployment))
	if err != nil {
		return err
	}

	if err = runtimeClient.Create(ctx, deploy); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create deployment: %v", err)
		}
	}

	// wait for helm controller to be ready
	logger.Info("waiting for helm controller")
	if err = waitForDeploymentReady(ctx, runtimeClient, logger); err != nil {
		return err
	}

	logger.Info("finished installing helm controller")

	return nil
}

func CheckHelmControllerExists(ctx context.Context, runtimeClient client.Client) (bool, error) {
	key := client.ObjectKey{
		Namespace: "boundless-system",
		Name:      "helm-controller",
	}
	if err := runtimeClient.Get(ctx, key, &v1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func waitForDeploymentReady(ctx context.Context, runtimeClient client.Client, log logr.Logger) error {
	key := client.ObjectKey{
		Namespace: "boundless-system",
		Name:      "helm-controller",
	}
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		d := &v1.Deployment{}
		if err := runtimeClient.Get(ctx, key, d); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if d.Status.AvailableReplicas == d.Status.Replicas {
			// Expected replicas active
			return true, nil
		}
		log.V(1).Info(fmt.Sprintf("waiting for helm controller to %d replicas, currently at %d", d.Status.Replicas, d.Status.AvailableReplicas))
		return false, nil
	})
}

func yamlToServiceAccount(yml []byte) (*corev1.ServiceAccount, error) {
	svcAcc := &corev1.ServiceAccount{}
	if err := yaml.Unmarshal(yml, svcAcc); err != nil {
		return nil, err
	}
	return svcAcc, nil
}
func yamlToCRD(yml []byte) (*apiextenv1.CustomResourceDefinition, error) {
	crd := &apiextenv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(yml, crd); err != nil {
		return nil, err
	}
	return crd, nil
}

func yamlToDeployment(yml []byte) (*v1.Deployment, error) {
	deploy := &v1.Deployment{}
	if err := yaml.Unmarshal(yml, deploy); err != nil {
		return nil, err
	}
	return deploy, nil
}

func yamlToClusterRole(yml []byte) (*rbacv1.ClusterRole, error) {
	role := &rbacv1.ClusterRole{}
	if err := yaml.Unmarshal(yml, role); err != nil {
		return nil, err
	}
	return role, nil
}

func yamlToClusterRoleBinding(yml []byte) (*rbacv1.ClusterRoleBinding, error) {
	roleBinding := &rbacv1.ClusterRoleBinding{}
	if err := yaml.Unmarshal(yml, roleBinding); err != nil {
		return nil, err
	}
	return roleBinding, nil
}
