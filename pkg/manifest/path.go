package manifest

import (
	"io/fs"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/mirantiscontainers/blueprint-operator/internal/template"
)

type reader func(fs.FS, string, interface{}) ([]*unstructured.Unstructured, error)

func read(fsys fs.FS, pathname string, r reader, cfg interface{}) ([]*unstructured.Unstructured, error) {
	var aggregated []*unstructured.Unstructured

	err := fs.WalkDir(fsys, pathname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}
		els, err := r(fsys, path, cfg)
		if err != nil {
			return err
		}

		aggregated = append(aggregated, els...)
		return err
	})
	if err != nil {
		return nil, err
	}
	return aggregated, nil
}

// Read reads YAML files into Unstructured objects from the provided files system.
// It reads a single file or all files in a directory (and its subdirectories) based on the provided pathname.
func Read(rfs fs.FS, pathname string) ([]*unstructured.Unstructured, error) {
	return read(rfs, pathname, readFile, nil)
}

// ReadTemplate read and parses YAML templates into Unstructured objects from the provided files system.
// It reads a single file or all files in a directory (and its subdirectories) based on the provided pathname.
func ReadTemplate(rfs fs.FS, pathname string, cfg interface{}) ([]*unstructured.Unstructured, error) {
	return read(rfs, pathname, parseTemplateFile, cfg)
}

// parseTemplateFile parses a single file as a template using the provided config struct
// and returns unstructured manifest objects.
func parseTemplateFile(rfs fs.FS, pathname string, cfg interface{}) ([]*unstructured.Unstructured, error) {
	buf, err := template.ParseFSTemplate(rfs, pathname, cfg)
	if err != nil {
		return nil, err
	}

	return Decode(buf)
}

// readFile parses a single file.
func readFile(rfs fs.FS, pathname string, _ interface{}) ([]*unstructured.Unstructured, error) {
	file, err := rfs.Open(pathname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Decode(file)
}
