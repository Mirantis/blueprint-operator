package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// UnstructuredReader an interface that all manifest readers should implement
type UnstructuredReader interface {
	Read() (*unstructured.Unstructured, error)
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

// ReadManifest decodes the whole manifest and return list of unstructured objects
func (m *manifestReader) ReadManifest() ([]*unstructured.Unstructured, error) {
	var o []*unstructured.Unstructured
	var errs error
	for {
		obj, err := m.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors.Join(errs, fmt.Errorf("could not read object: %w", err))
			continue
		}
		if obj == nil {
			continue
		}

		o = append(o, obj)
	}

	return o, errs
}
