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

package insights_test

import (
	"context"
	"fmt"
	"strconv"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/controller/test"
	"github.com/RedHatInsights/runtimes-inventory-operator/pkg/insights"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type setupTestInput struct {
	client      ctrlclient.Client
	objs        []ctrlclient.Object
	opNamespace string
	integration *insights.InsightsIntegration
	*test.TestUtilsConfig
	*test.InsightsTestResources
}

var _ = Describe("InsightsIntegration", func() {
	var t *setupTestInput

	count := 0
	namespaceWithSuffix := func(name string) string {
		return name + "-" + strconv.Itoa(count)
	}

	Describe("setting up", func() {
		BeforeEach(func() {
			t = &setupTestInput{
				TestUtilsConfig: &test.TestUtilsConfig{
					EnvInsightsEnabled:       &[]bool{true}[0],
					EnvInsightsBackendDomain: &[]string{"insights.example.com"}[0],
					EnvInsightsProxyImageTag: &[]string{"example.com/proxy:latest"}[0],
				},
				InsightsTestResources: &test.InsightsTestResources{
					Namespace:       namespaceWithSuffix("setup-test"),
					UserAgentPrefix: "test-operator/0.0.0",
				},
			}
			t.objs = []ctrlclient.Object{
				t.NewNamespace(),
				t.NewOperatorDeployment(),
			}
			t.opNamespace = t.Namespace
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

			manager := test.NewFakeManager(t.client, s, &logger)
			deploy := t.NewOperatorDeployment()
			t.integration = insights.NewInsightsIntegration(manager, deploy.Name, t.opNamespace, t.UserAgentPrefix, &logger)
			t.integration.OSUtils = test.NewTestOSUtils(t.TestUtilsConfig)
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

		Context("with defaults", func() {
			It("should return proxy URL", func() {
				result, err := t.integration.Setup()
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.String()).To(Equal(fmt.Sprintf("http://%s.%s.svc:8080", t.NewInsightsProxyService().Name, t.Namespace)))
			})

			It("should create config map", func() {
				_, err := t.integration.Setup()
				Expect(err).ToNot(HaveOccurred())

				expected := t.NewProxyConfigMap()
				actual := &corev1.ConfigMap{}
				err = t.client.Get(context.Background(), types.NamespacedName{
					Name:      expected.Name,
					Namespace: expected.Namespace,
				}, actual)
				Expect(err).ToNot(HaveOccurred())

				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Annotations).To(Equal(expected.Annotations))
				Expect(metav1.IsControlledBy(actual, t.getOperatorDeployment())).To(BeTrue())
				Expect(actual.Data).To(BeEmpty())
			})
		})

		Context("with Insights disabled", func() {
			BeforeEach(func() {
				t.EnvInsightsEnabled = &[]bool{false}[0]
				t.objs = append(t.objs,
					t.NewProxyConfigMap(),
				)
			})

			It("should return nil", func() {
				result, err := t.integration.Setup()
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should delete config map", func() {
				_, err := t.integration.Setup()
				Expect(err).ToNot(HaveOccurred())

				expected := t.NewProxyConfigMap()
				actual := &corev1.ConfigMap{}
				err = t.client.Get(context.Background(), types.NamespacedName{
					Name:      expected.Name,
					Namespace: expected.Namespace,
				}, actual)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue(), err.Error())
			})
		})

		Context("when run out-of-cluster", func() {
			BeforeEach(func() {
				t.opNamespace = ""
			})

			It("should return nil", func() {
				result, err := t.integration.Setup()
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should not create config map", func() {
				_, err := t.integration.Setup()
				Expect(err).ToNot(HaveOccurred())

				expected := t.NewProxyConfigMap()
				actual := &corev1.ConfigMap{}
				err = t.client.Get(context.Background(), types.NamespacedName{
					Name:      expected.Name,
					Namespace: expected.Namespace,
				}, actual)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue(), err.Error())
			})
		})
	})
})

func (t *setupTestInput) getOperatorDeployment() *appsv1.Deployment {
	deploy := &appsv1.Deployment{}
	expected := t.NewOperatorDeployment()
	err := t.client.Get(context.Background(), types.NamespacedName{
		Name:      expected.Name,
		Namespace: expected.Namespace,
	}, deploy)
	Expect(err).ToNot(HaveOccurred())
	return deploy
}
