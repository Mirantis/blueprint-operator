package utils

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateNamespaceIfNotExist checks if provided namespace exists, and creates it if it does not exist.
func CreateNamespaceIfNotExist(runtimeClient client.Client, ctx context.Context, logger logr.Logger, namespace string) error {
	ns := corev1.Namespace{}
	err := runtimeClient.Get(ctx, client.ObjectKey{Name: namespace}, &ns)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("namespace does not exist, creating", "Namespace", namespace)
			ns.ObjectMeta.Name = namespace
			err = runtimeClient.Create(ctx, &ns)
			if err != nil {
				return err
			}

		} else {
			logger.Info("error checking namespace exists", "Namespace", namespace)
			return err
		}
	}

	return nil
}
