package manifest

import (
	"io/fs"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Read reads YAML files into Unstructured objects from the provided files system.
// It reads a single file or all files in a directory (and its subdirectories) based on the provided pathname.
func Read(fsys fs.FS, pathname string) ([]*unstructured.Unstructured, error) {
	var aggregated []*unstructured.Unstructured

	err := fs.WalkDir(fsys, pathname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}
		els, err := readFile(fsys, path)
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

// readFile parses a single file.
func readFile(rfs fs.FS, pathname string) ([]*unstructured.Unstructured, error) {
	file, err := rfs.Open(pathname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Decode(file)
}
