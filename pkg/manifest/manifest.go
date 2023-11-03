package manifest

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	core_v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

type ManifestController struct {
	client client.Client
	logger logr.Logger
}

func NewManifestController(client client.Client, logger logr.Logger) *ManifestController {
	return &ManifestController{
		client: client,
		logger: logger,
	}
}

func (mc *ManifestController) Deserialize(data []byte) (*client.Object, error) {
	apiextensionsv1.AddToScheme(scheme.Scheme)
	apiextensionsv1beta1.AddToScheme(scheme.Scheme)
	decoder := scheme.Codecs.UniversalDeserializer()

	runtimeObject, groupVersionKind, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	mc.logger.Info("Decode details", "runtimeObject", runtimeObject, "groupVersionKind", groupVersionKind)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch groupVersionKind.Kind {
	case "Namespace":
		// @TODO: create the namespace if it doesn't exist
		namespaceObj := convertToNamespaceObject(runtimeObject)
		mc.logger.Info("Creating namespace", "Namespace", namespaceObj.Name)
		err = mc.client.Create(ctx, namespaceObj)
		if err != nil {
			return nil, err
		}
		mc.logger.Info("Namespace created successfully:", "Namespace", namespaceObj.Name)
	default:
		mc.logger.Info("Object kind not supported", "Kind", groupVersionKind.Kind)
	}

	return nil, nil
}

func convertToNamespaceObject(obj runtime.Object) *core_v1.Namespace {
	myobj := obj.(*core_v1.Namespace)
	return myobj
}
