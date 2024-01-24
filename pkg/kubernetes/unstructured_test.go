package kubernetes

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReadManifest", func() {
	Context("ManifestTest", func() {
		var (
			rawConfigMap = []byte(`apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns`)
			rawMultiManifest = []byte(`apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns
---
apiVersion: v1
kind: Deployment
metadata:
  name: test-dep
  namespace: test-ns
---
apiVersion: v1
kind: Service
metadata:
  name: test-svc
  namespace: test-ns
`)
		)
		Context("manifest readers testing", func() {
			It("Should read manifest correctly", func() {
				objs, err := NewManifestReader(rawConfigMap).ReadManifest()
				Expect(err).NotTo(HaveOccurred())

				Expect(len(objs)).Should(Equal(1))

				// Tests to ensure validity of object
				Expect(objs[0].GetName()).To(Equal("test-cm"))
				Expect(objs[0].GetNamespace()).To(Equal("test-ns"))
			})
			It("Should read manifest containing multiple objects correctly", func() {
				objs, err := NewManifestReader(rawMultiManifest).ReadManifest()
				Expect(err).NotTo(HaveOccurred())

				Expect(len(objs)).Should(Equal(3))
				Expect(objs[0].GetName()).To(Equal("test-cm"))
				Expect(objs[1].GetName()).To(Equal("test-dep"))
				Expect(objs[2].GetName()).To(Equal("test-svc"))
			})
		})
	})
})
