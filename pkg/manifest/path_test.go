package manifest

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestRead(t *testing.T) {
	testfs := fstest.MapFS{
		// file at root
		"dep001.yaml": {Data: toYaml(getDeploy("dep001"))},

		"crds":           {Mode: fs.ModeDir},
		"crds/crd1.yaml": {Data: toYaml(getCRD("test1.example.com"))},
		"crds/crd2.yaml": {Data: toYaml(getCRD("test2.example.com"))},
		"crds/crd3.yaml": {Data: toYaml(getCRD("test3.example.com"))},
		"crds/crd4.yaml": {Data: toYaml(getCRD("test4.example.com"))},

		"manifests":           {Mode: fs.ModeDir},
		"manifests/dep1.yaml": {Data: toYaml(getDeploy("dep1"))},
		"manifests/dep2.yaml": {Data: toYaml(getDeploy("dep2"))},

		"subdir":                 {Mode: fs.ModeDir},
		"subdir/dir1":            {Mode: fs.ModeDir},
		"subdir/dir1/dep11.yaml": {Data: toYaml(getDeploy("dep11"))},
		"subdir/dir1/dep12.yaml": {Data: toYaml(getDeploy("dep12"))},
		"subdir/dir1/dep13.yaml": {Data: toYaml(getDeploy("dep13"))},
		"subdir/dir2":            {Mode: fs.ModeDir},
		"subdir/dir2/dep21.yaml": {Data: toYaml(getDeploy("dep21"))},
		"subdir/dir2/dep22.yaml": {Data: toYaml(getDeploy("dep22"))},

		"empty": {Mode: fs.ModeDir},
	}

	tests := []struct {
		name      string
		fsys      fs.FS
		pathname  string
		recursive bool
		count     int
	}{
		{
			name:     "Read resources from single file at root",
			pathname: "dep001.yaml",
			fsys:     testfs,
			count:    1,
		},
		{
			name:     "Read resources from a directory",
			pathname: "crds",
			fsys:     testfs,
			count:    4,
		},
		{
			name:     "Read resources from a directory",
			pathname: "manifests",
			fsys:     testfs,
			count:    2,
		},
		{
			name:     "Read resources from all files and subdirectories from given directory",
			pathname: "subdir",
			fsys:     testfs,
			count:    5,
		},
		{
			name:     "Read resources from all files and subdirectories from root",
			pathname: ".",
			fsys:     testfs,
			count:    12,
		},
		{
			name:     "Read resources from an empty directory",
			pathname: "empty",
			fsys:     testfs,
			count:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Read(tt.fsys, tt.pathname)
			assert.NoError(t, err)
			assert.Equal(t, tt.count, len(got), "Read() count = %v, want %v", len(got), tt.count)
		})
	}
}

func getDeploy(name string) appsv1.Deployment {
	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/appsv1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
		},
	}
}

func getCRD(name string) apiextensionsv1.CustomResourceDefinition {
	return apiextensionsv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/appsv1", Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "test.example.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:     "MyExamples",
				Singular:   "MyExample",
				Kind:       "MyExample",
				ShortNames: []string{"me"},
				ListKind:   "MyExampleList",
				Categories: []string{"all"},
			},
			Scope:                 apiextensionsv1.NamespaceScoped,
			PreserveUnknownFields: false,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Subresources: &apiextensionsv1.CustomResourceSubresources{
						Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
						Scale: &apiextensionsv1.CustomResourceSubresourceScale{
							SpecReplicasPath:   ".spec.num.num1",
							StatusReplicasPath: ".status.num.num2",
						},
					},
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"content": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"key": {Type: "string"},
									},
								},
								"num": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"num1": {Type: "integer"},
										"num2": {Type: "integer"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func toYaml(obj interface{}) []byte {
	b, err := yaml.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return b
}
