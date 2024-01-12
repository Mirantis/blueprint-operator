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
	ReadManifest() ([]*unstructured.Unstructured, error)
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

// ReadManifest decodes the whole manifest and return list of unstructured objects
func (m *manifestReader) ReadManifest() ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured

	for {
		var o unstructured.Unstructured
		if err := m.decoder.Decode(&o); err != nil {
			if err != io.EOF {
				return objs, fmt.Errorf("error decoding yaml manifest file: %s", err)
			}
			break
		}
		objs = append(objs, &o)

	}
	return objs, nil
}
