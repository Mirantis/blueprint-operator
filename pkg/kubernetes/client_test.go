package kubernetes

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClient_Apply(t *testing.T) {
	// Create a fake client
	fakeClient := fake.NewFakeClient()
	k8sClient := NewClient(logr.Discard(), fakeClient)

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
	err := k8sClient.Apply(context.Background(), service)
	assert.NoError(t, err)

	// Check if the Service was created
	actual := &corev1.Service{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, actual)
	assert.NoError(t, err)
	assert.Equal(t, service.Name, actual.Name)
	assert.Equal(t, service.Namespace, actual.Namespace)
}

func TestClient_ApplyExisting(t *testing.T) {
	// This test now fails because fake client does not support client.ApplyPatchType
	// https://github.com/kubernetes/kubernetes/issues/115598
	// Either, we need to use a real client, or use client.MergePatchType
	t.Skip()

	// Create a fake client
	fakeClient := fake.NewFakeClient()
	k8sClient := NewClient(logr.Discard(), fakeClient)

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

	err := fakeClient.Create(context.Background(), service)
	assert.NoError(t, err)

	// Call the Apply method
	err = k8sClient.Apply(context.Background(), service)
	assert.NoError(t, err)

	// Check if the Service was created
	actual := &corev1.Service{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, actual)
	assert.NoError(t, err)
	assert.Equal(t, service.Name, actual.Name)
	assert.Equal(t, service.Namespace, actual.Namespace)
}

func TestClient_Delete(t *testing.T) {
	// Create a fake client
	fakeClient := fake.NewFakeClient()
	k8sClient := NewClient(logr.Discard(), fakeClient)

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

	err := fakeClient.Create(context.Background(), service)
	assert.NoError(t, err)

	err = k8sClient.Delete(context.Background(), service)
	assert.NoError(t, err)

	// Check if the Service was created
	actual := &corev1.Service{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, actual)
	assert.True(t, apierrors.IsNotFound(err))
}
