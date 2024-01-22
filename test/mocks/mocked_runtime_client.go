package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type MockClient struct{ mock.Mock }

func (m *MockClient) List(ctx context.Context, list ctrlClient.ObjectList, opts ...ctrlClient.ListOption) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) Create(ctx context.Context, obj ctrlClient.Object, opts ...ctrlClient.CreateOption) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) Delete(ctx context.Context, obj ctrlClient.Object, opts ...ctrlClient.DeleteOption) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) Update(ctx context.Context, obj ctrlClient.Object, opts ...ctrlClient.UpdateOption) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) Patch(ctx context.Context, obj ctrlClient.Object, patch ctrlClient.Patch, opts ...ctrlClient.PatchOption) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) DeleteAllOf(ctx context.Context, obj ctrlClient.Object, opts ...ctrlClient.DeleteAllOfOption) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) Status() ctrlClient.SubResourceWriter {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) SubResource(subResource string) ctrlClient.SubResourceClient {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) Scheme() *runtime.Scheme {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) RESTMapper() meta.RESTMapper {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

// NewMockClient returns a MockClient that is a mocked sigs.k8s.io/controller-runtime/pkg/client
// and can be used to mock calls to the Kubernetes API during unit tests
func NewMockClient() *MockClient { return &MockClient{} }

func (m *MockClient) Get(ctx context.Context, key ctrlClient.ObjectKey, obj ctrlClient.Object, opts ...ctrlClient.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}
