package manifest

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	boundlessv1alpha1 "github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
	"github.com/mirantiscontainers/boundless-operator/test/mocks"
)

var _ = Describe("Status", func() {
	var (
		m        *mocks.MockClient
		mc       *ManifestController
		manifest types.NamespacedName
		logger   logr.Logger
	)

	BeforeEach(func() {
		m = mocks.NewMockClient()
		logger = log.FromContext(context.TODO())
		mc = NewManifestController(m, logger)

		manifest = types.NamespacedName{
			Namespace: "namespace",
			Name:      "manifest",
		}
	})

	Context("ErrorTest", func() {
		Context("No manifest objects", func() {
			It("Should return unhealthy status", func() {
				stat, err := mc.CheckManifestStatus(context.TODO(), logger, manifest, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat).Should(Equal(Status{boundlessv1alpha1.TypeComponentUnhealthy, "No objects detected for manifest", ""}))
			})
		})

		Context("Error when retrieving deployment belonging to manifest", func() {
			It("Should return unhealthy status", func() {
				manifestObjects := []boundlessv1alpha1.ManifestObject{
					{
						Kind:      "Deployment",
						Name:      "TestDeployment",
						Namespace: "TestNamespace",
					},
				}

				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDeployment"},
					&v1.Deployment{},
					mock.Anything,
				).Return(fmt.Errorf("error"))

				stat, err := mc.CheckManifestStatus(context.TODO(), logger, manifest, manifestObjects)
				Expect(err).To(HaveOccurred())
				Expect(stat).Should(Equal(Status{boundlessv1alpha1.TypeComponentUnhealthy, "Unable to get deployment from manifest", ""}))
			})

		})
	})

	Context("Manifest still Progressing", func() {
		Context("Single deployment manifest still progressing", func() {
			It("Should return manifest status as still progressing", func() {
				manifestObjects := []boundlessv1alpha1.ManifestObject{
					{
						Kind:      "Deployment",
						Name:      "TestDeployment",
						Namespace: "TestNamespace",
					},
				}

				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDeployment"},
					&v1.Deployment{},
					mock.Anything,
				).Return(nil).Run(func(args mock.Arguments) {
					deployment := args.Get(2).(*v1.Deployment)
					deployment.Status.Conditions = append(deployment.Status.Conditions, v1.DeploymentCondition{Type: v1.DeploymentProgressing})
				})

				stat, err := mc.CheckManifestStatus(context.TODO(), logger, manifest, manifestObjects)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat).Should(Equal(Status{boundlessv1alpha1.TypeComponentProgressing, "1 or more manifest deployments are still progressing", ""}))
			})
		})

		Context("Deployment is available but Daemonset still progressing", func() {
			It("Should return manifest status as still progressing", func() {
				manifestObjects := []boundlessv1alpha1.ManifestObject{
					{
						Kind:      "Deployment",
						Name:      "TestDeployment",
						Namespace: "TestNamespace",
					},
					{
						Kind:      "DaemonSet",
						Name:      "TestDaemonset",
						Namespace: "TestNamespace",
					},
				}

				// Mock out that Deployment is Done
				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDeployment"},
					&v1.Deployment{},
					mock.Anything,
				).Return(nil).Run(func(args mock.Arguments) {
					deployment := args.Get(2).(*v1.Deployment)
					deployment.Status.Conditions = append(deployment.Status.Conditions, v1.DeploymentCondition{Type: v1.DeploymentAvailable})
				})

				// Mock out Daemonset still Progressing
				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDaemonset"},
					&v1.DaemonSet{},
					mock.Anything,
				).Return(nil).Run(func(args mock.Arguments) {
					daemonset := args.Get(2).(*v1.DaemonSet)
					daemonset.Status.NumberAvailable = 0
					daemonset.Status.NumberReady = 0
					daemonset.Status.DesiredNumberScheduled = 1
				})

				stat, err := mc.CheckManifestStatus(context.TODO(), logger, manifest, manifestObjects)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat).Should(Equal(Status{boundlessv1alpha1.TypeComponentProgressing, "1 or more manifest daemonsets are still progressing", ""}))
			})
		})

		Context("Daemonset is available but Deployment is still progressing", func() {
			It("Should return manifest status as still progressing", func() {
				manifestObjects := []boundlessv1alpha1.ManifestObject{
					{
						Kind:      "Deployment",
						Name:      "TestDeployment",
						Namespace: "TestNamespace",
					},
					{
						Kind:      "DaemonSet",
						Name:      "TestDaemonset",
						Namespace: "TestNamespace",
					},
				}

				// Mock out that Deployment is Done
				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDeployment"},
					&v1.Deployment{},
					mock.Anything,
				).Return(nil).Run(func(args mock.Arguments) {
					deployment := args.Get(2).(*v1.Deployment)
					deployment.Status.Conditions = append(deployment.Status.Conditions, v1.DeploymentCondition{Type: v1.DeploymentProgressing})
				})

				// Mock out Daemonset still Available
				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDaemonset"},
					&v1.DaemonSet{},
					mock.Anything,
				).Return(nil).Run(func(args mock.Arguments) {
					daemonset := args.Get(2).(*v1.DaemonSet)
					daemonset.Status.NumberAvailable = 1
					daemonset.Status.NumberReady = 1
					daemonset.Status.DesiredNumberScheduled = 1
				})

				stat, err := mc.CheckManifestStatus(context.TODO(), logger, manifest, manifestObjects)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat).Should(Equal(Status{boundlessv1alpha1.TypeComponentProgressing, "1 or more manifest deployments are still progressing", ""}))
			})
		})
	})

	Context("Manifest is Available", func() {
		Context("Deployment & Daemonset are both available", func() {
			It("Should return manifest status as available", func() {
				manifestObjects := []boundlessv1alpha1.ManifestObject{
					{
						Kind:      "Deployment",
						Name:      "TestDeployment",
						Namespace: "TestNamespace",
					},
					{
						Kind:      "DaemonSet",
						Name:      "TestDaemonset",
						Namespace: "TestNamespace",
					},
				}

				// Mock out that Deployment is Done
				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDeployment"},
					&v1.Deployment{},
					mock.Anything,
				).Return(nil).Run(func(args mock.Arguments) {
					deployment := args.Get(2).(*v1.Deployment)
					deployment.Status.Conditions = append(deployment.Status.Conditions, v1.DeploymentCondition{Type: v1.DeploymentAvailable})
				})

				// Mock out Daemonset is Done
				m.On("Get",
					context.TODO(),
					types.NamespacedName{Namespace: "TestNamespace", Name: "TestDaemonset"},
					&v1.DaemonSet{},
					mock.Anything,
				).Return(nil).Run(func(args mock.Arguments) {
					daemonset := args.Get(2).(*v1.DaemonSet)
					daemonset.Status.NumberAvailable = 1
					daemonset.Status.NumberReady = 1
					daemonset.Status.DesiredNumberScheduled = 1
				})

				stat, err := mc.CheckManifestStatus(context.TODO(), logger, manifest, manifestObjects)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat).Should(Equal(Status{
					boundlessv1alpha1.TypeComponentAvailable,
					"Manifest Components Available",
					"Deployments : Manifest Deployments Available, Daemonsets : Manifest Daemonsets Available",
				}))

			})
		})
	})

})
