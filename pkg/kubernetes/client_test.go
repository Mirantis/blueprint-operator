package kubernetes

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"
)

func TestClient_Apply(t *testing.T) {
	// Create a fake client
	fclient := fake.NewFakeClient()

	// Create a new Client
	c := NewClient(logr.Discard(), fclient)

	// Create a new Service
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "test-service",
			},
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	// Call the Apply method
	err := c.Apply(context.Background(), service)
	assert.NoError(t, err)

	// Check if the Service was created
	actual := &corev1.Service{}
	err = fclient.Get(context.Background(), client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, actual)
	assert.NoError(t, err)
	assert.Equal(t, service.Name, actual.Name)
	assert.Equal(t, service.Namespace, actual.Namespace)
}

func TestClient_ApplyExisting(t *testing.T) {
	// Create a fake client
	fclient := fake.NewFakeClient()

	// Create a new Client
	c := NewClient(logr.Discard(), fclient)

	// Create a new Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "test-service",
			},
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	err := fclient.Create(context.Background(), service)
	assert.NoError(t, err)

	// Call the Apply method
	err = c.Apply(context.Background(), service)
	assert.NoError(t, err)

	// Check if the Service was created
	actual := &corev1.Service{}
	err = fclient.Get(context.Background(), client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, actual)
	assert.NoError(t, err)
	assert.Equal(t, service.Name, actual.Name)
	assert.Equal(t, service.Namespace, actual.Namespace)
}

func TestClient_Delete(t *testing.T) {
	// Create a fake client
	fclient := fake.NewFakeClient()

	// Create a new Client
	c := NewClient(logr.Discard(), fclient)

	// Create a new Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "test-service",
			},
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	err := fclient.Create(context.Background(), service)
	assert.NoError(t, err)

	err = c.Delete(context.Background(), service)
	assert.NoError(t, err)

	// Check if the Service was created
	actual := &corev1.Service{}
	err = fclient.Get(context.Background(), client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, actual)
	assert.True(t, apierrors.IsNotFound(err))
}
