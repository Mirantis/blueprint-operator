package components

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForCRDs waits for the given CRDs to be available in the cluster.
func WaitForCRDs(ctx context.Context, c client.Client, logger logr.Logger, crdNames []string) error {
	logger.Info("Waiting for CRDs to be available", "crds", crdNames)
	conditionFunc := func(ctx context.Context) (done bool, err error) {
		for _, crdName := range crdNames {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
		}
		// All CRDs are available
		return true, nil
	}

	err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, conditionFunc)
	if err != nil {
		return fmt.Errorf("failed to wait for CRDs to be available: %w", err)
	}
	logger.Info("CRDs are available")

	return nil
}
