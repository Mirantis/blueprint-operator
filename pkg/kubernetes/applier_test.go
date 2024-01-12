package kubernetes

import (
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func makeManifest(objs ...client.Object) []byte {
	var out bytes.Buffer
	for _, obj := range objs {
		data, err := yaml.Marshal(obj)
		Expect(err).NotTo(HaveOccurred())
		out.Write(data)
		out.WriteString("---\n")
	}
	return out.Bytes()
}

var _ = Describe("Apply", func() {
	var (
		c       client.Client
		applier *Applier
	)

	BeforeEach(func() {
		c = fake.NewFakeClient()
		logger := ctrl.Log.WithName("test")
		applier = NewApplier(logger, c)
	})

	Context("ApplierTest", func() {
		Context("Apply", func() {
			It("Should create objects correctly", func() {

				deploy := v1.Deployment{
					TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-dep", Namespace: "test-ns"},
					Spec: v1.DeploymentSpec{
						Replicas: int32Ptr(2),
					},
				}

				manifest := makeManifest(&deploy)
				Expect(applier.Apply(context.TODO(), NewManifestReader(manifest))).To(Succeed())

				var actual v1.Deployment
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-dep", Namespace: "test-ns"}, &actual)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual.Name).Should(Equal(deploy.Name))
				Expect(actual.Namespace).Should(Equal(deploy.Namespace))
				Expect(actual.Spec.Replicas).Should(Equal(deploy.Spec.Replicas))
			})

			It("Should create multiple objects correctly", func() {
				deploy := v1.Deployment{
					TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-dep", Namespace: "test-ns"},
					Spec: v1.DeploymentSpec{
						Replicas: int32Ptr(2),
					},
				}
				svc := corev1.Service{
					TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "test-ns"},
					Spec: corev1.ServiceSpec{
						Type: "ClusterIP",
					},
				}

				manifest := makeManifest(&deploy, &svc)
				Expect(applier.Apply(context.TODO(), NewManifestReader(manifest))).To(Succeed())

				var actualDep v1.Deployment
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-dep", Namespace: "test-ns"}, &actualDep)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualDep.Name).Should(Equal(deploy.Name))
				Expect(actualDep.Namespace).Should(Equal(deploy.Namespace))
				Expect(actualDep.Spec.Replicas).Should(Equal(deploy.Spec.Replicas))

				var actualSvc corev1.Service
				err = c.Get(context.TODO(), client.ObjectKey{Name: "test-svc", Namespace: "test-ns"}, &actualSvc)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualSvc.Name).Should(Equal(svc.Name))
				Expect(actualSvc.Namespace).Should(Equal(svc.Namespace))
				Expect(actualSvc.Spec.Type).Should(Equal(svc.Spec.Type))
			})
		})
		Context("Update", func() {
			It("Should update objects correctly", func() {

				deploy := v1.Deployment{
					TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-dep", Namespace: "test-ns"},
					Spec: v1.DeploymentSpec{
						Replicas: int32Ptr(2),
					},
				}
				Expect(c.Create(context.TODO(), &deploy)).To(Succeed())

				deploy.Spec.Replicas = int32Ptr(4)
				manifest := makeManifest(&deploy)
				Expect(applier.Apply(context.TODO(), NewManifestReader(manifest))).To(Succeed())

				var actual v1.Deployment
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-dep", Namespace: "test-ns"}, &actual)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual.Spec.Replicas).Should(Equal(int32Ptr(4)))
			})
		})

		Context("Delete", func() {
			It("Should delete manifest objects correctly", func() {

				deploy := v1.Deployment{
					TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-dep", Namespace: "test-ns"},
					Spec: v1.DeploymentSpec{
						Replicas: int32Ptr(2),
					},
				}
				Expect(c.Create(context.TODO(), &deploy)).To(Succeed())

				manifest := makeManifest(&deploy)
				reader := NewManifestReader(manifest)
				objs, err := reader.ReadManifest()
				Expect(err).ToNot(HaveOccurred())

				Expect(applier.Delete(context.TODO(), objs)).To(Succeed())
				var actual v1.Deployment
				err = c.Get(context.TODO(), client.ObjectKey{Name: "test-dep", Namespace: "test-ns"}, &actual)
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})

func int32Ptr(val int32) *int32 {
	return &val
}
