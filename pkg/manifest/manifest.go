package manifest

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	core_v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ManifestController struct {
	client client.Client
	logger logr.Logger
}

type NamespaceObject struct {
	ApiVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
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

	mc.logger.Info("Sakshi:: details of RuntimeObject", "runtimeObject", runtimeObject, "groupVersionKind", groupVersionKind)

	/*if _, ok := runtimeObject.(*NamespaceObject); !ok {
		mc.logger.Info("Extract Name from RuntimeObject", "Name", (runtimeObject.(*NamespaceObject)).Name)
	}*/

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	mc.logger.Info("Sakshi::: printing groupVersionKind.Kind", "Kind", groupVersionKind.Kind)

	switch groupVersionKind.Kind {
	case "Namespace":

		// Create a namespace object
		/*if err := execTemplate(options, namespaceTmpl, path.Join(base, "namespace.yaml")); err != nil {
		    return fmt.Errorf("generate namespace failed: %w", err)
		}*/
		ns := createNamespaceObject("development")

		mc.logger.Info("Creating namespace", "Namespace", "development")
		err = mc.client.Create(ctx, ns)
		if err != nil {
			return nil, err
		}
		mc.logger.Info("Namespace created successfully:", "Namespace", "development")
	}

	return nil, nil
}

func createNamespaceObject(namespace string) *core_v1.Namespace {
	return &core_v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

/*func execTemplate(obj interface{}, tmpl, filename string) error {
	t, err := template.New("tmpl").Parse(tmpl)
	if err != nil {
		return err
	}

	var data bytes.Buffer
	writer := bufio.NewWriter(&data)
	if err := t.Execute(writer, obj); err != nil {
		return err
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.WriteString(file, data.String())
	if err != nil {
		return err
	}

	return file.Sync()
}*/
