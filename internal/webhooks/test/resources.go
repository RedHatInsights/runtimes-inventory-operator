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

package test

import (
	"fmt"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/controller/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AgentWebhookTestResources struct {
	*test.InsightsTestResources
}

func (r *AgentWebhookTestResources) NewPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "insights-agent-webhook-test",
			Namespace: r.Namespace,
			Labels: map[string]string{
				"com.redhat.insights.runtimes/inject-agent": "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "example.com/test:latest",
					Env: []corev1.EnvVar{
						{
							Name:  "TEST",
							Value: "some-value",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &[]bool{false}[0],
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{
								"ALL",
							},
						},
					},
				},
			},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &[]bool{true}[0],
			},
		},
	}
}

func (r *AgentWebhookTestResources) NewPodMultiContainer() *corev1.Pod {
	pod := r.NewPod()
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:  "other",
		Image: "example.com/other:latest",
		Env: []corev1.EnvVar{
			{
				Name:  "OTHER",
				Value: "some-other-value",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: &[]bool{false}[0],
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
	})
	return pod
}

func (r *AgentWebhookTestResources) NewPodJavaToolOptions() *corev1.Pod {
	pod := r.NewPod()
	container := &pod.Spec.Containers[0]
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  "JAVA_TOOL_OPTIONS",
			Value: "-Dexisting=var",
		})
	return pod
}

func (r *AgentWebhookTestResources) NewPodJavaToolOptionsFrom() *corev1.Pod {
	pod := r.NewPod()
	container := &pod.Spec.Containers[0]
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name: "JAVA_TOOL_OPTIONS",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-config",
					},
					Key:      "java-tool-options",
					Optional: &[]bool{true}[0],
				},
			},
		})
	return pod
}

func (r *AgentWebhookTestResources) NewPodOtherNamespace(namespace string) *corev1.Pod {
	pod := r.NewPod()
	pod.Namespace = namespace
	return pod
}

func (r *AgentWebhookTestResources) NewPodNoInjectLabel() *corev1.Pod {
	pod := r.NewPod()
	delete(pod.Labels, "com.redhat.insights.runtimes/inject-agent")
	return pod
}

func (r *AgentWebhookTestResources) NewPodLogDebugLabel() *corev1.Pod {
	pod := r.NewPod()
	pod.Labels["com.redhat.insights.runtimes/log-debug"] = "true"
	return pod
}

func (r *AgentWebhookTestResources) NewPodLogDebugLabelBad() *corev1.Pod {
	pod := r.NewPod()
	pod.Labels["com.redhat.insights.runtimes/log-debug"] = "not-a-bool"
	return pod
}

func (r *AgentWebhookTestResources) NewPodContainerLabel() *corev1.Pod {
	pod := r.NewPodMultiContainer()
	pod.Labels["com.redhat.insights.runtimes/container"] = "other"
	return pod
}

func (r *AgentWebhookTestResources) NewPodContainerBadLabel() *corev1.Pod {
	pod := r.NewPodMultiContainer()
	pod.Labels["com.redhat.insights.runtimes/container"] = "wrong"
	return pod
}

func (r *AgentWebhookTestResources) NewPodJavaOptsVar() *corev1.Pod {
	pod := r.NewPod()
	pod.Labels["com.redhat.insights.runtimes/java-options-var"] = "SOME_OTHER_VAR"
	return pod
}

type mutatedPodOptions struct {
	debugLog         bool
	javaOptionsName  string
	javaOptionsValue string
	namespace        string
	image            string
	pullPolicy       corev1.PullPolicy
	scheme           string
	resources        *corev1.ResourceRequirements
	// Function to produce mutated container array
	containersFunc func(*AgentWebhookTestResources, *mutatedPodOptions) []corev1.Container
}

func (r *AgentWebhookTestResources) setDefaultMutatedPodOptions(options *mutatedPodOptions) {
	if len(options.namespace) == 0 {
		options.namespace = r.Namespace
	}
	if len(options.image) == 0 {
		options.image = "quay.io/ebaron/runtimes-agent-init:latest" // FIXME
	}
	if len(options.pullPolicy) == 0 {
		options.pullPolicy = corev1.PullAlways
	}
	if len(options.javaOptionsName) == 0 {
		options.javaOptionsName = "JAVA_TOOL_OPTIONS"
	}
	if options.resources == nil {
		options.resources = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
		}
	}
	options.scheme = "http"
	if options.containersFunc == nil {
		options.containersFunc = newMutatedContainers
	}
}

func (r *AgentWebhookTestResources) NewMutatedPod() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{})
}

func (r *AgentWebhookTestResources) NewMutatedPodJavaToolOptions() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{
		javaOptionsValue: "-Dexisting=var ",
	})
}

func (r *AgentWebhookTestResources) NewMutatedPodCustomImage() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{
		image:      "example.com/agent-init:2.0.0",
		pullPolicy: corev1.PullIfNotPresent,
	})
}

func (r *AgentWebhookTestResources) NewMutatedPodCustomDevImage() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{
		image:      "example.com/agent-init:latest",
		pullPolicy: corev1.PullAlways,
	})
}

func (r *AgentWebhookTestResources) NewMutatedPodLogDebugLabel() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{
		debugLog: true,
	})
}

func (r *AgentWebhookTestResources) NewMutatedPodMultiContainer() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{
		containersFunc: newMutatedMultiContainers,
	})
}

func (r *AgentWebhookTestResources) NewMutatedPodContainerLabel() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{
		containersFunc: newMutatedMultiContainersLabel,
	})
}

func (r *AgentWebhookTestResources) NewMutatedPodJavaOptsVarLabel() *corev1.Pod {
	return r.newMutatedPod(&mutatedPodOptions{
		javaOptionsName: "SOME_OTHER_VAR",
	})
}

func (r *AgentWebhookTestResources) newMutatedPod(options *mutatedPodOptions) *corev1.Pod {
	r.setDefaultMutatedPodOptions(options)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "insights-agent-webhook-test",
			Namespace: options.namespace,
			Labels: map[string]string{
				"com.redhat.insights.runtimes/inject-agent": "true",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:            "runtimes-agent-init",
					Image:           options.image,
					ImagePullPolicy: options.pullPolicy,
					Command:         []string{"cp", "-v", "/agent/runtimes-agent.jar", "/tmp/rh-runtimes-insights/runtimes-agent.jar"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "runtimes-agent-init",
							MountPath: "/tmp/rh-runtimes-insights",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{
								"ALL",
							},
						},
					},
					Resources: *options.resources,
				},
			},
			Containers: options.containersFunc(r, options),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &[]bool{true}[0],
			},
			Volumes: []corev1.Volume{
				{
					Name: "runtimes-agent-init",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: &[]resource.Quantity{resource.MustParse("50Mi")}[0],
						},
					},
				},
			},
		},
	}

	return pod
}

func newMutatedContainers(r *AgentWebhookTestResources, options *mutatedPodOptions) []corev1.Container {
	containers := r.NewPodMultiContainer().Spec.Containers
	return []corev1.Container{*r.newMutatedContainer(&containers[0], options)}
}

func newMutatedMultiContainers(r *AgentWebhookTestResources, options *mutatedPodOptions) []corev1.Container {
	containers := r.NewPodMultiContainer().Spec.Containers
	return []corev1.Container{*r.newMutatedContainer(&containers[0], options), containers[1]}
}

func newMutatedMultiContainersLabel(r *AgentWebhookTestResources, options *mutatedPodOptions) []corev1.Container {
	containers := r.NewPodMultiContainer().Spec.Containers
	return []corev1.Container{containers[0], *r.newMutatedContainer(&containers[1], options)}
}

func (r *AgentWebhookTestResources) newMutatedContainer(original *corev1.Container, options *mutatedPodOptions) *corev1.Container {
	svc := r.NewInsightsProxyService()
	debugLogging := ""
	if options.debugLog {
		debugLogging = ";debug=true"
	}
	agentArgLine := fmt.Sprintf("-javaagent:'/tmp/rh-runtimes-insights/runtimes-agent.jar=name=$(RHT_INSIGHTS_JAVA_AGENT_POD_NAME);base_url=http://%s.%s.svc:8080;pod_name=$(RHT_INSIGHTS_JAVA_AGENT_POD_NAME);pod_namespace=$(RHT_INSIGHTS_JAVA_AGENT_POD_NAMESPACE);token=unused%s'",
		svc.Name, svc.Namespace, debugLogging)
	prependEnv := append([]corev1.EnvVar{
		{
			Name: "RHT_INSIGHTS_JAVA_AGENT_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
		{
			Name: "RHT_INSIGHTS_JAVA_AGENT_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.namespace",
				},
			},
		},
	}, original.Env...)
	container := &corev1.Container{
		Name:  original.Name,
		Image: original.Image,
		Env: append(prependEnv, corev1.EnvVar{
			Name:  options.javaOptionsName,
			Value: options.javaOptionsValue + agentArgLine,
		}),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: &[]bool{false}[0],
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "runtimes-agent-init",
				MountPath: "/tmp/rh-runtimes-insights",
				ReadOnly:  true,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
		},
	}

	return container
}
