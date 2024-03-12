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

package controllers

import (
	"context"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/controllers/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type insightsUnitTestInput struct {
	client     ctrlclient.Client
	controller *InsightsReconciler
	objs       []ctrlclient.Object
	*test.TestUtilsConfig
	*test.InsightsTestResources
}

var _ = Describe("InsightsController", func() {
	var t *insightsUnitTestInput

	Describe("configuring watches", func() {

		BeforeEach(func() {
			t = &insightsUnitTestInput{
				TestUtilsConfig: &test.TestUtilsConfig{
					EnvInsightsEnabled:       &[]bool{true}[0],
					EnvInsightsBackendDomain: &[]string{"insights.example.com"}[0],
					EnvInsightsProxyImageTag: &[]string{"example.com/proxy:latest"}[0],
				},
				InsightsTestResources: &test.InsightsTestResources{
					Namespace: "test",
				},
			}
			t.objs = []ctrlclient.Object{
				t.NewNamespace(),
				t.NewGlobalPullSecret(),
				t.NewOperatorDeployment(),
			}
		})

		JustBeforeEach(func() {
			s := scheme.Scheme
			logger := zap.New()
			logf.SetLogger(logger)

			t.client = fake.NewClientBuilder().WithScheme(s).WithObjects(t.objs...).Build()

			config := &InsightsReconcilerConfig{
				Client:    t.client,
				Scheme:    s,
				Log:       logger,
				Namespace: t.Namespace,
				OSUtils:   test.NewTestOSUtils(t.TestUtilsConfig),
			}
			controller, err := NewInsightsReconciler(config)
			Expect(err).ToNot(HaveOccurred())
			t.controller = controller
		})

		Context("for secrets", func() {
			It("should reconcile global pull secret", func() {
				result := t.controller.isPullSecretOrProxyConfig(context.Background(), t.NewGlobalPullSecret())
				Expect(result).To(ConsistOf(t.deploymentReconcileRequest()))
			})
			It("should reconcile APICast secret", func() {
				result := t.controller.isPullSecretOrProxyConfig(context.Background(), t.NewInsightsProxySecret())
				Expect(result).To(ConsistOf(t.deploymentReconcileRequest()))
			})
			It("should not reconcile a secret in another namespace", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.NewGlobalPullSecret().Name,
						Namespace: "other",
					},
				}
				result := t.controller.isPullSecretOrProxyConfig(context.Background(), secret)
				Expect(result).To(BeEmpty())
			})
		})

		Context("for deployments", func() {
			It("should reconcile proxy deployment", func() {
				result := t.controller.isProxyDeployment(context.Background(), t.NewInsightsProxyDeployment())
				Expect(result).To(ConsistOf(t.deploymentReconcileRequest()))
			})
			It("should not reconcile a deployment in another namespace", func() {
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.NewInsightsProxyDeployment().Name,
						Namespace: "other",
					},
				}
				result := t.controller.isProxyDeployment(context.Background(), deploy)
				Expect(result).To(BeEmpty())
			})
		})

		Context("for services", func() {
			It("should reconcile proxy service", func() {
				result := t.controller.isProxyService(context.Background(), t.NewInsightsProxyService())
				Expect(result).To(ConsistOf(t.deploymentReconcileRequest()))
			})
			It("should not reconcile a service in another namespace", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.NewInsightsProxyService().Name,
						Namespace: "other",
					},
				}
				result := t.controller.isProxyService(context.Background(), svc)
				Expect(result).To(BeEmpty())
			})
		})
	})
})

func (t *insightsUnitTestInput) deploymentReconcileRequest() reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: "insights-proxy", Namespace: t.Namespace}}
}
