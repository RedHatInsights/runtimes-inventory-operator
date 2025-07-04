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

package common

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	InsightsConfigMapNamePrefix = "insights-proxy"
	ProxyDeploymentNamePrefix   = InsightsConfigMapNamePrefix
	ProxyServiceNamePrefix      = ProxyDeploymentNamePrefix
	ProxyServicePort            = 8080
	ProxySecretNamePrefix       = "apicastconf"
	EnvInsightsBackendDomain    = "INSIGHTS_BACKEND_DOMAIN"
	EnvInsightsProxyDomain      = "INSIGHTS_PROXY_DOMAIN"
	EnvInsightsEnabled          = "INSIGHTS_ENABLED"
	// Environment variable to override the Insights proxy image
	EnvInsightsProxyImageTag = "RELATED_IMAGE_INSIGHTS_PROXY"
	// ALL capability to drop for restricted pod security. See:
	// https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
	CapabilityAll corev1.Capability = "ALL"
)
