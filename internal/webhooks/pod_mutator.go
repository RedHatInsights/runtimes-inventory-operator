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

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/common"
)

type podMutator struct {
	client client.Client
	log    *logr.Logger
	config *AgentWebhookConfig
}

var _ admission.CustomDefaulter = &podMutator{}

const (
	agentLabelPrefix         = "com.redhat.insights.runtimes/"
	agentLabelInjectAgent    = agentLabelPrefix + "inject-agent"
	agentLabelContainer      = agentLabelPrefix + "container"
	agentLabelJavaOptionsVar = agentLabelPrefix + "java-options-var"
	agentLabelLogDebug       = agentLabelPrefix + "log-debug"

	javaAgentParam           = "-javaagent"
	agentPath                = "/tmp/rh-runtimes-insights/runtimes-agent.jar"
	identNameEnvVar          = "RHT_INSIGHTS_JAVA_IDENTIFICATION_NAME"
	tokenEnvVar              = "RHT_INSIGHTS_JAVA_AUTH_TOKEN"
	uploadURLEnvVar          = "RHT_INSIGHTS_JAVA_UPLOAD_BASE_URL"
	podNameEnvVar            = "RHT_INSIGHTS_JAVA_AGENT_POD_NAME"
	podNamespaceEnvVar       = "RHT_INSIGHTS_JAVA_AGENT_POD_NAMESPACE"
	debugEnvVar              = "RHT_INSIGHTS_JAVA_AGENT_DEBUG"
	defaultAgentInitImageTag = "registry.redhat.io/rh-lightspeed-runtimes/runtimes-agent-init-rhel9:latest"
	agentMaxSizeBytes        = "50Mi"
	agentInitCpuRequest      = "10m"
	agentInitMemoryRequest   = "32Mi"
	defaultJavaOptsVar       = "JAVA_TOOL_OPTIONS"
)

// Default optionally mutates a pod to inject the Insights Java Agent
func (r *podMutator) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected a Pod, but received a %T", obj)
	}
	// Use GenerateName for logging if no explicit Name is given
	podName := pod.Name
	if len(podName) == 0 {
		podName = pod.GenerateName
	}
	log := r.log.WithValues("name", podName, "namespace", pod.Namespace)

	// Check for required label and return early if missing.
	// This should not happen because such pods are filtered out by Kubernetes server-side due to our object selector.
	if !metav1.HasLabel(pod.ObjectMeta, agentLabelInjectAgent) {
		log.Info("pod is missing required label")
		return nil
	}

	if !isOptionEnabled(pod.Labels, agentLabelInjectAgent) {
		log.Info("agent injection is disabled")
		return nil
	}

	// Select target container
	container, err := getTargetContainer(pod)
	if err != nil {
		return err
	}

	// Add init container
	nonRoot := true
	imageTag := r.getImageTag()
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:            "runtimes-agent-init",
		Image:           imageTag,
		ImagePullPolicy: common.GetPullPolicy(imageTag),
		Command:         []string{"cp", "-v", "/agent/runtimes-agent.jar", "/tmp/rh-runtimes-insights/runtimes-agent.jar"},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "runtimes-agent-init",
				MountPath: "/tmp/rh-runtimes-insights",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: &nonRoot,
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					common.CapabilityAll,
				},
			},
		},
		Resources: *getResourceRequirements(),
	})

	// Add emptyDir volume to copy agent into, and mount it
	sizeLimit := resource.MustParse(agentMaxSizeBytes)
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "runtimes-agent-init",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: &sizeLimit,
			},
		},
	})

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "runtimes-agent-init",
		MountPath: "/tmp/rh-runtimes-insights",
		ReadOnly:  true,
	})

	// Set agent configuration using environment variables to avoid
	// shell escape issues with argument separators
	envs := []corev1.EnvVar{
		{
			Name: identNameEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  tokenEnvVar,
			Value: "unused",
		},
		{
			Name:  uploadURLEnvVar,
			Value: r.config.InsightsURL.String(),
		},
		{
			Name: podNameEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: podNamespaceEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
	if isOptionEnabled(pod.Labels, agentLabelLogDebug) {
		envs = append(envs, corev1.EnvVar{
			Name:  debugEnvVar,
			Value: "true",
		})
	}

	container.Env = append(container.Env, envs...)

	// Inject agent using JAVA_TOOL_OPTIONS or specified variable, appending to any existing value
	extended, err := r.extendJavaOptsVar(container.Env, getJavaOptionsVar(pod.Labels))
	if err != nil {
		return err
	}
	container.Env = extended

	log.Info("configured Red Hat Insights for Runtimes agent for pod")
	return nil
}

func getResourceRequirements() *corev1.ResourceRequirements {
	resources := &corev1.ResourceRequirements{}
	// TODO allow customization
	common.PopulateResourceRequest(resources, agentInitCpuRequest, agentInitMemoryRequest)
	return resources
}

func (r *podMutator) getImageTag() string {
	// Lazily look up image tag
	if r.config.InitImageTag == nil {
		agentInitImage := r.config.GetEnvOrDefault(agentInitImageTagEnv, defaultAgentInitImageTag)
		r.config.InitImageTag = &agentInitImage
	}
	return *r.config.InitImageTag
}

func getTargetContainer(pod *corev1.Pod) (*corev1.Container, error) {
	if len(pod.Spec.Containers) == 0 {
		// Should never happen, Kubernetes doesn't allow this
		return nil, errors.New("pod has no containers")
	}
	label, pres := pod.Labels[agentLabelContainer]
	if !pres {
		// Use the first container by default
		return &pod.Spec.Containers[0], nil
	}
	// Find the container matching the label
	return findNamedContainer(pod.Spec.Containers, label)
}

func findNamedContainer(containers []corev1.Container, name string) (*corev1.Container, error) {
	for i, container := range containers {
		if container.Name == name {
			return &containers[i], nil
		}
	}
	return nil, fmt.Errorf("no container found with name \"%s\"", name)
}

func getJavaOptionsVar(labels map[string]string) string {
	result := defaultJavaOptsVar
	value, pres := labels[agentLabelJavaOptionsVar]
	if pres {
		result = value
	}
	return result
}

func (r *podMutator) extendJavaOptsVar(envs []corev1.EnvVar, javaOptsVar string) ([]corev1.EnvVar, error) {
	existing, err := findJavaOptsVar(envs, javaOptsVar)
	if err != nil {
		return nil, err
	}

	agentArgLine := fmt.Sprintf("%s:%s", javaAgentParam, agentPath)
	if existing != nil {
		existing.Value += " " + agentArgLine
	} else {
		envs = append(envs, corev1.EnvVar{
			Name:  javaOptsVar,
			Value: agentArgLine,
		})
	}

	return envs, nil
}

func findJavaOptsVar(envs []corev1.EnvVar, javaOptsVar string) (*corev1.EnvVar, error) {
	for i, env := range envs {
		if env.Name == javaOptsVar {
			if env.ValueFrom != nil {
				return nil, fmt.Errorf("environment variable %s uses \"valueFrom\" and cannot be extended", javaOptsVar)
			}
			return &envs[i], nil
		}
	}
	return nil, nil
}

func isOptionEnabled(labels map[string]string, key string) bool {
	return strings.ToLower(labels[key]) == "true"
}

func newAgentArg(name string, value string) string {
	return name + "=" + value
}
