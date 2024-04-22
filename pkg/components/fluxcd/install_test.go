package fluxcd

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_installCRDs(t *testing.T) {
	fakeClient := fake.NewFakeClient()
	err := installCRDs(fakeClient, logr.Discard())
	assert.NoError(t, err)
}

func Test_installComponents(t *testing.T) {
	fakeClient := fake.NewFakeClient()
	err := installComponents(fakeClient, logr.Discard())
	assert.NoError(t, err)

	clusterRole := v1.ClusterRole{}

	err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: "crd-controller"}, &clusterRole)
	assert.NoError(t, err)
	assert.NotEmpty(t, clusterRole.Name)
}
