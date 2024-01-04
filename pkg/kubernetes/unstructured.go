package kubernetes

import (
	"bytes"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// UnstructuredReader an interface that all manifest readers should implement
type UnstructuredReader interface {
	Read() (*unstructured.Unstructured, error)
}

// NewManifestReader initializes a reader for yaml manifests
func NewManifestReader(manifest []byte) UnstructuredReader {
	return &manifestReader{
		decoder:  yaml.NewYAMLToJSONDecoder(bytes.NewReader(manifest)),
		manifest: manifest,
	}
}

// manifestReader is an unstructured reader that contains a JSONDecoder
type manifestReader struct {
	decoder  *yaml.YAMLToJSONDecoder
	manifest []byte
}

// Read decodes a single yaml object into an unstructured object
func (m *manifestReader) Read() (*unstructured.Unstructured, error) {
	// loop to skip empty yaml objects
	var o *unstructured.Unstructured
	for {
		err := m.decoder.Decode(o)
		if err == io.EOF {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("error '%w' decoding manifest: %s", err, string(m.manifest))
		}

		if o == nil {
			continue
		}
		return o, nil
	}
}
