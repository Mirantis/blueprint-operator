package manifest

import (
	"io/fs"
	"os"
	"path"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Read reads YAML files into Unstructured objects from the provided files system.
// It reads a single file or all files in a directory based on the provided pathname.
func Read(fsys fs.FS, pathname string, recursive bool) ([]*unstructured.Unstructured, error) {
	objs, err := read(fsys, pathname, recursive)
	if err != nil {
		return nil, err
	}
	return objs, nil
}

// read parses a single file or all files in a directory.
func read(fsys fs.FS, pathname string, recursive bool) ([]*unstructured.Unstructured, error) {
	info, err := fs.Stat(fsys, pathname)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return readDir(fsys, pathname, recursive)
	}
	return readFile(fsys, pathname)
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

// readDir parses all files in a single directory
func readDir(rfs fs.FS, pathname string, recursive bool) ([]*unstructured.Unstructured, error) {
	list, err := fs.ReadDir(rfs, pathname)
	if err != nil {
		return nil, err
	}

	var aggregated []*unstructured.Unstructured
	for _, f := range list {
		name := path.Join(pathname, f.Name())
		pathDirOrFile, err := fs.Stat(rfs, name)
		var els []*unstructured.Unstructured

		if os.IsNotExist(err) || os.IsPermission(err) {
			return aggregated, err
		}

		switch {
		case pathDirOrFile.IsDir() && recursive:
			els, err = readDir(rfs, name, recursive)
		case !pathDirOrFile.IsDir():
			els, err = readFile(rfs, name)
		}

		if err != nil {
			return nil, err
		}
		aggregated = append(aggregated, els...)
	}
	return aggregated, nil
}
