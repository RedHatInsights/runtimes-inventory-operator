// Copyright The Cryostat Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller_test

import (
	"context"
	"strconv"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/controller"
	"github.com/RedHatInsights/runtimes-inventory-operator/internal/controller/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type insightsTestInput struct {
	client      ctrlclient.Client
	controller  *controller.InsightsReconciler
	objs        []ctrlclient.Object
	opNamespace string
	*test.TestUtilsConfig
	*test.InsightsTestResources
}

var _ = Describe("InsightsController", func() {
	var t *insightsTestInput

	count := 0
	namespaceWithSuffix := func(name string) string {
		return name + "-" + strconv.Itoa(count)
	}

	Describe("reconciling a request", func() {
		BeforeEach(func() {
			t = &insightsTestInput{
				TestUtilsConfig: &test.TestUtilsConfig{
					EnvInsightsEnabled:       &[]bool{true}[0],
					EnvInsightsBackendDomain: &[]string{"insights.example.com"}[0],
					EnvInsightsProxyImageTag: &[]string{"example.com/proxy:latest"}[0],
				},
				InsightsTestResources: &test.InsightsTestResources{
					Namespace:       namespaceWithSuffix("controller-test"),
					UserAgentPrefix: "test-operator/0.0.0",
				},
			}
			t.objs = []ctrlclient.Object{
				t.NewNamespace(),
				t.NewGlobalPullSecret(),
				t.NewClusterVersion(),
				t.NewOperatorDeployment(),
				t.NewProxyConfigMap(),
			}
		})

		JustBeforeEach(func() {
			s := scheme.Scheme
			logger := zap.New()
			logf.SetLogger(logger)

			t.client = k8sClient
			for _, obj := range t.objs {
				err := t.client.Create(context.Background(), obj)
				Expect(err).ToNot(HaveOccurred())
			}

			config := &controller.InsightsReconcilerConfig{
				Client:          t.client,
				Scheme:          s,
				Log:             logger,
				Namespace:       t.Namespace,
				UserAgentPrefix: t.UserAgentPrefix,
				OperatorName:    t.NewOperatorDeployment().Name,
				OSUtils:         test.NewTestOSUtils(t.TestUtilsConfig),
			}
			controller, err := controller.NewInsightsReconciler(config)
			Expect(err).ToNot(HaveOccurred())
			t.controller = controller
		})

		JustAfterEach(func() {
			for _, obj := range t.objs {
				err := ctrlclient.IgnoreNotFound(t.client.Delete(context.Background(), obj))
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			count++
		})

		Context("successfully creates required resources", func() {
			Context("with defaults", func() {
				JustBeforeEach(func() {
					result, err := t.reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})
				It("should create the APICast config secret", func() {
					expected := t.NewInsightsProxySecret()
					actual := &corev1.Secret{}
					err := t.client.Get(context.Background(), types.NamespacedName{
						Name:      expected.Name,
						Namespace: expected.Namespace,
					}, actual)
					Expect(err).ToNot(HaveOccurred())

					Expect(actual.Labels).To(Equal(expected.Labels))
					Expect(actual.Annotations).To(Equal(expected.Annotations))
					Expect(metav1.IsControlledBy(actual, t.getProxyConfigMap())).To(BeTrue())
					Expect(actual.Data).To(HaveLen(1))
					Expect(actual.Data).To(HaveKey("config.json"))
					Expect(actual.Data["config.json"]).To(MatchJSON(expected.Data["config.json"]))
				})
				It("should create the proxy deployment", func() {
					expected := t.NewInsightsProxyDeployment()
					actual := &appsv1.Deployment{}
					err := t.client.Get(context.Background(), types.NamespacedName{
						Name:      expected.Name,
						Namespace: expected.Namespace,
					}, actual)
					Expect(err).ToNot(HaveOccurred())

					t.checkProxyDeployment(actual, expected)
				})
				It("should create the proxy service", func() {
					expected := t.NewInsightsProxyService()
					actual := &corev1.Service{}
					err := t.client.Get(context.Background(), types.NamespacedName{
						Name:      expected.Name,
						Namespace: expected.Namespace,
					}, actual)
					Expect(err).ToNot(HaveOccurred())

					Expect(actual.Labels).To(Equal(expected.Labels))
					Expect(actual.Annotations).To(Equal(expected.Annotations))
					Expect(metav1.IsControlledBy(actual, t.getProxyConfigMap())).To(BeTrue())

					Expect(actual.Spec.Selector).To(Equal(expected.Spec.Selector))
					Expect(actual.Spec.Type).To(Equal(expected.Spec.Type))
					Expect(actual.Spec.Ports).To(ConsistOf(expected.Spec.Ports))
				})
			})
			Context("with a proxy domain", func() {
				BeforeEach(func() {
					t.EnvInsightsProxyDomain = &[]string{"proxy.example.com"}[0]
					t.WithProxy = true
				})
				JustBeforeEach(func() {
					result, err := t.reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})
				It("should create the APICast config secret", func() {
					expected := t.NewInsightsProxySecret()
					actual := &corev1.Secret{}
					err := t.client.Get(context.Background(), types.NamespacedName{
						Name:      expected.Name,
						Namespace: expected.Namespace,
					}, actual)
					Expect(err).ToNot(HaveOccurred())

					Expect(actual.Labels).To(Equal(expected.Labels))
					Expect(actual.Annotations).To(Equal(expected.Annotations))
					Expect(metav1.IsControlledBy(actual, t.getProxyConfigMap())).To(BeTrue())
					Expect(actual.Data).To(HaveLen(1))
					Expect(actual.Data).To(HaveKey("config.json"))
					Expect(actual.Data["config.json"]).To(MatchJSON(expected.Data["config.json"]))
				})
				It("should create the proxy deployment", func() {
					expected := t.NewInsightsProxyDeployment()
					actual := &appsv1.Deployment{}
					err := t.client.Get(context.Background(), types.NamespacedName{
						Name:      expected.Name,
						Namespace: expected.Namespace,
					}, actual)
					Expect(err).ToNot(HaveOccurred())

					t.checkProxyDeployment(actual, expected)
				})
			})
		})
		Context("updating the deployment", func() {
			BeforeEach(func() {
				t.objs = append(t.objs,
					t.NewInsightsProxyDeployment(),
					t.NewInsightsProxySecret(),
					t.NewInsightsProxyService(),
				)
			})
			Context("with resource requirements", func() {
				var resources *corev1.ResourceRequirements

				BeforeEach(func() {
					resources = &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					}
				})
				JustBeforeEach(func() {
					// Fetch the deployment
					deploy := t.getProxyDeployment()

					// Change the resource requirements
					deploy.Spec.Template.Spec.Containers[0].Resources = *resources

					// Update the deployment
					err := t.client.Update(context.Background(), deploy)
					Expect(err).ToNot(HaveOccurred())

					// Reconcile again
					result, err := t.reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})
				It("should leave the custom resource requirements", func() {
					// Fetch the deployment again
					actual := t.getProxyDeployment()

					// Check only resource requirements differ from defaults
					t.Resources = resources
					expected := t.NewInsightsProxyDeployment()
					t.checkProxyDeployment(actual, expected)
				})
			})
		})
	})
})

func (t *insightsTestInput) reconcile() (reconcile.Result, error) {
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "insights-proxy", Namespace: t.Namespace}}
	return t.controller.Reconcile(context.Background(), req)
}

func (t *insightsTestInput) getProxyDeployment() *appsv1.Deployment {
	deploy := t.NewInsightsProxyDeployment()
	err := t.client.Get(context.Background(), types.NamespacedName{
		Name:      deploy.Name,
		Namespace: deploy.Namespace,
	}, deploy)
	Expect(err).ToNot(HaveOccurred())
	return deploy
}

func (t *insightsTestInput) checkProxyDeployment(actual, expected *appsv1.Deployment) {
	Expect(actual.Labels).To(Equal(expected.Labels))
	Expect(actual.Annotations).To(Equal(expected.Annotations))
	Expect(metav1.IsControlledBy(actual, t.getProxyConfigMap())).To(BeTrue())
	Expect(actual.Spec.Selector).To(Equal(expected.Spec.Selector))

	expectedTemplate := expected.Spec.Template
	actualTemplate := actual.Spec.Template
	Expect(actualTemplate.Labels).To(Equal(expectedTemplate.Labels))
	Expect(actualTemplate.Annotations).To(Equal(expectedTemplate.Annotations))
	Expect(actualTemplate.Spec.SecurityContext).To(Equal(expectedTemplate.Spec.SecurityContext))
	Expect(actualTemplate.Spec.Volumes).To(Equal(expectedTemplate.Spec.Volumes))

	Expect(actualTemplate.Spec.Containers).To(HaveLen(1))
	expectedContainer := expectedTemplate.Spec.Containers[0]
	actualContainer := actualTemplate.Spec.Containers[0]
	Expect(actualContainer.Ports).To(ConsistOf(expectedContainer.Ports))
	Expect(actualContainer.Env).To(ConsistOf(expectedContainer.Env))
	Expect(actualContainer.EnvFrom).To(ConsistOf(expectedContainer.EnvFrom))
	Expect(actualContainer.VolumeMounts).To(ConsistOf(expectedContainer.VolumeMounts))
	Expect(actualContainer.LivenessProbe).To(Equal(expectedContainer.LivenessProbe))
	Expect(actualContainer.StartupProbe).To(Equal(expectedContainer.StartupProbe))
	Expect(actualContainer.SecurityContext).To(Equal(expectedContainer.SecurityContext))

	test.ExpectResourceRequirements(&actualContainer.Resources, &expectedContainer.Resources)
}

func (t *insightsTestInput) getProxyConfigMap() *corev1.ConfigMap {
	cm := &corev1.ConfigMap{}
	expected := t.NewProxyConfigMap()
	err := t.client.Get(context.Background(), types.NamespacedName{
		Name:      expected.Name,
		Namespace: expected.Namespace,
	}, cm)
	Expect(err).ToNot(HaveOccurred())
	return cm
}
