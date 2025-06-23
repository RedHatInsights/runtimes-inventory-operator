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

package webhooks_test

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/common"
	"github.com/RedHatInsights/runtimes-inventory-operator/internal/controller/test"
	webhooktests "github.com/RedHatInsights/runtimes-inventory-operator/internal/webhooks/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type defaulterTestInput struct {
	client ctrlclient.Client
	objs   []ctrlclient.Object
	*webhooktests.AgentWebhookTestResources
}

var _ = Describe("PodDefaulter", func() {
	var t *defaulterTestInput
	count := 0

	namespaceWithSuffix := func(name string) string {
		return name + "-agent-" + strconv.Itoa(count)
	}

	BeforeEach(func() {
		ns := namespaceWithSuffix("test")
		t = &defaulterTestInput{
			AgentWebhookTestResources: &webhooktests.AgentWebhookTestResources{
				InsightsTestResources: &test.InsightsTestResources{
					Namespace: ns,
				},
			},
		}
		t.objs = []ctrlclient.Object{
			t.NewNamespace(),
		}
	})

	JustBeforeEach(func() {
		logger := zap.New()
		logf.SetLogger(logger)

		t.client = k8sClient
		for _, obj := range t.objs {
			err := t.client.Create(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	JustAfterEach(func() {
		for _, obj := range t.objs {
			err := ctrlclient.IgnoreNotFound(t.client.Delete(ctx, obj))
			Expect(err).ToNot(HaveOccurred())
		}
	})

	AfterEach(func() {
		count++
	})

	Context("configuring a pod", func() {
		var originalPod *corev1.Pod
		var expectedPod *corev1.Pod

		BeforeEach(func() {
			svc := t.NewInsightsProxyService()
			insightsURL, err := url.Parse(fmt.Sprintf("http://%s.%s.svc:8080", svc.Name, svc.Namespace))
			Expect(err).ToNot(HaveOccurred())
			agentWebhookConfig.InsightsURL = insightsURL
		})

		JustBeforeEach(func() {
			err := t.client.Create(ctx, originalPod)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			agentWebhookConfig.InsightsURL = nil
			originalPod = nil
			expectedPod = nil
		})

		ExpectPod := func() {
			It("should add init container", func() {
				actual := t.getPod(expectedPod)
				expectedInitContainers := expectedPod.Spec.InitContainers
				Expect(actual.Spec.InitContainers).To(HaveLen(len(expectedInitContainers)))
				for idx := range expectedInitContainers {
					expected := expectedPod.Spec.InitContainers[idx]
					container := actual.Spec.InitContainers[idx]
					Expect(container.Name).To(Equal(expected.Name))
					Expect(container.Command).To(Equal(expected.Command))
					Expect(container.Args).To(Equal(expected.Args))
					Expect(container.Env).To(Equal(expected.Env))
					Expect(container.EnvFrom).To(Equal(expected.EnvFrom))
					Expect(container.Image).To(HavePrefix(expected.Image[:strings.Index(expected.Image, ":")]))
					Expect(container.VolumeMounts).To(Equal(expected.VolumeMounts))
					Expect(container.SecurityContext).To(Equal(expected.SecurityContext))
					Expect(container.Ports).To(Equal(expected.Ports))
					Expect(container.LivenessProbe).To(Equal(expected.LivenessProbe))
					Expect(container.ReadinessProbe).To(Equal(expected.ReadinessProbe))
					test.ExpectResourceRequirements(&container.Resources, &expected.Resources)
				}
			})

			It("should add volume(s)", func() {
				actual := t.getPod(expectedPod)
				Expect(actual.Spec.Volumes).To(ConsistOf(expectedPod.Spec.Volumes))
			})

			It("should add volume mounts(s)", func() {
				actual := t.getPod(expectedPod)
				Expect(actual.Spec.Containers).To(HaveLen(len(expectedPod.Spec.Containers)))
				for i, expected := range expectedPod.Spec.Containers {
					container := actual.Spec.Containers[i]
					Expect(container.VolumeMounts).To(ConsistOf(expected.VolumeMounts))
				}
			})

			It("should add environment variables", func() {
				actual := t.getPod(expectedPod)
				Expect(actual.Spec.Containers).To(HaveLen(len(expectedPod.Spec.Containers)))
				for i, expected := range expectedPod.Spec.Containers {
					container := actual.Spec.Containers[i]
					Expect(container.Env).To(ConsistOf(expected.Env))
					Expect(container.EnvFrom).To(ConsistOf(expected.EnvFrom))
				}
			})

			It("should add ports(s)", func() {
				actual := t.getPod(expectedPod)
				Expect(actual.Spec.Containers).To(HaveLen(len(expectedPod.Spec.Containers)))
				for i, expected := range expectedPod.Spec.Containers {
					container := actual.Spec.Containers[i]
					Expect(container.Ports).To(ConsistOf(expected.Ports))
				}
			})
		}

		Context("with defaults", func() {
			BeforeEach(func() {
				originalPod = t.NewPod()
				expectedPod = t.NewMutatedPod()
			})

			ExpectPod()
		})

		Context("with existing JAVA_TOOL_OPTIONS", func() {
			BeforeEach(func() {
				originalPod = t.NewPodJavaToolOptions()
				expectedPod = t.NewMutatedPodJavaToolOptions()
			})

			ExpectPod()
		})

		Context("with existing JAVA_TOOL_OPTIONS using valueFrom", func() {
			BeforeEach(func() {
				originalPod = t.NewPodJavaToolOptionsFrom()
				// Should fail
				expectedPod = originalPod
			})

			ExpectPod()
		})

		Context("with no inject label", func() {
			BeforeEach(func() {
				originalPod = t.NewPodNoInjectLabel()
				// Should fail
				expectedPod = originalPod
			})

			ExpectPod()
		})

		Context("with custom image tag", func() {
			var saveOSUtils common.OSUtils

			BeforeEach(func() {
				originalPod = t.NewPod()
			})

			setImageTag := func(imageTag string) {
				saveOSUtils = agentWebhookConfig.OSUtils
				// Force webhook to query environment again
				agentWebhookConfig.InitImageTag = nil
				agentWebhookConfig.OSUtils = test.NewTestOSUtils(&test.TestUtilsConfig{
					EnvAgentInitImageTag: &[]string{imageTag}[0],
				})
			}

			JustAfterEach(func() {
				// Reset state
				agentWebhookConfig.OSUtils = saveOSUtils
				agentWebhookConfig.InitImageTag = nil
			})

			Context("for development", func() {
				BeforeEach(func() {
					expectedPod = t.NewMutatedPodCustomDevImage()
					setImageTag("example.com/agent-init:latest")
				})

				ExpectPod()

				It("should use Always pull policy", func() {
					actual := t.getPod(expectedPod)
					expectedInitContainers := expectedPod.Spec.InitContainers
					Expect(actual.Spec.InitContainers).To(HaveLen(len(expectedInitContainers)))
					for idx := range expectedInitContainers {
						container := actual.Spec.InitContainers[idx]
						Expect(container.ImagePullPolicy).To(Equal(corev1.PullAlways))
					}
				})
			})

			Context("for release", func() {
				BeforeEach(func() {
					expectedPod = t.NewMutatedPodCustomImage()
					setImageTag("example.com/agent-init:2.0.0")
				})

				ExpectPod()

				It("should use IfNotPresent pull policy", func() {
					actual := t.getPod(expectedPod)
					expectedInitContainers := expectedPod.Spec.InitContainers
					Expect(actual.Spec.InitContainers).To(HaveLen(len(expectedInitContainers)))
					for idx := range expectedInitContainers {
						container := actual.Spec.InitContainers[idx]
						Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
					}
				})
			})
		})

		Context("with a debug logging label", func() {
			Context("enabled", func() {
				BeforeEach(func() {
					originalPod = t.NewPodLogDebugLabel()
					expectedPod = t.NewMutatedPodLogDebugLabel()
				})

				ExpectPod()
			})

			Context("that is invalid", func() {
				BeforeEach(func() {
					originalPod = t.NewPodLogDebugLabelBad()
					// Should fail
					expectedPod = originalPod
				})

				ExpectPod()
			})
		})

		Context("with multiple containers", func() {
			BeforeEach(func() {
				originalPod = t.NewPodMultiContainer()
				expectedPod = t.NewMutatedPodMultiContainer()
			})

			ExpectPod()
		})

		Context("with a custom container label", func() {
			Context("for a container that exists", func() {
				BeforeEach(func() {
					originalPod = t.NewPodContainerLabel()
					expectedPod = t.NewMutatedPodContainerLabel()
				})

				ExpectPod()
			})

			Context("for a container that doesn't exist", func() {
				BeforeEach(func() {
					originalPod = t.NewPodContainerBadLabel()
					// Should fail
					expectedPod = originalPod
				})

				ExpectPod()
			})
		})

		Context("with a custom java options var label", func() {
			BeforeEach(func() {
				originalPod = t.NewPodJavaOptsVar()
				expectedPod = t.NewMutatedPodJavaOptsVarLabel()
			})

			ExpectPod()
		})

		// TODO Add test cases for custom resource requirements when implemented
	})
})

func (t *defaulterTestInput) getPod(expected *corev1.Pod) *corev1.Pod {
	pod := &corev1.Pod{}
	err := t.client.Get(context.Background(), types.NamespacedName{Name: expected.Name, Namespace: expected.Namespace}, pod)
	Expect(err).ToNot(HaveOccurred())
	return pod
}
