package manifest

import (
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Decode reads a stream of YAML documents from the given reader and returns them as a slice of Unstructured objects.
func Decode(reader io.Reader) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLToJSONDecoder(reader)
	var objs []*unstructured.Unstructured
	var err error
	for {
		out := unstructured.Unstructured{}
		err = decoder.Decode(&out)
		if err != nil {
			break
		}
		if len(out.Object) == 0 {
			continue
		}
		objs = append(objs, &out)
	}
	if err != io.EOF {
		return nil, err
	}
	return objs, nil
}
