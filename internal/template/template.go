// Package template provides utilities for parsing and Go templates.
package template

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"path"
)

func executeTemplate(t *template.Template, cfg interface{}) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	err := t.Execute(buf, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to execute template: %s", err)
	}

	return buf, nil
}

// ParseTemplate receives an input template and a config struct and returns the
// executed template.
func ParseTemplate(sourceTmpl string, cfg interface{}) (*bytes.Buffer, error) {
	tmpl, err := template.New("dummyTemplate").Parse(sourceTmpl)
	if err != nil {
		return nil, fmt.Errorf("unable to parse template: %s", err)
	}

	return executeTemplate(tmpl, cfg)
}

// ParseFSTemplate parses template from the supplied file system at the specified path
// using the supplied config struct and returns the executed template.
func ParseFSTemplate(sourceTmpl fs.FS, pathname string, cfg interface{}) (*bytes.Buffer, error) {
	tmpl, err := template.New(path.Base(pathname)).ParseFS(sourceTmpl, pathname)
	if err != nil {
		return nil, fmt.Errorf("unable to parse template: %s", err)
	}

	return executeTemplate(tmpl, cfg)
}
