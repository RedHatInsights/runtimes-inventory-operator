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

package insights

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/common"
	"github.com/RedHatInsights/runtimes-inventory-operator/internal/controller"
	"github.com/RedHatInsights/runtimes-inventory-operator/internal/webhooks"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// InsightsIntegration allows your operator to manage a proxy
// for sending Red Hat Insights reports from Java-based workloads
// to the Runtimes Inventory service.
type InsightsIntegration struct {
	Manager         ctrl.Manager
	Log             *logr.Logger
	opName          string
	opNamespace     string
	userAgentPrefix string
	common.OSUtils
}

// NewInsightsIntegration creates a new InsightsIntegration using
// your operator's Manager and logger.
// Provide the operator's name and namespace,
// which can be discovered using the Kubernetes downward API.
// The User Agent prefix must be an approved UHC Auth Proxy prefix.
func NewInsightsIntegration(mgr ctrl.Manager, operatorName string, operatorNamespace string, userAgentPrefix string, log *logr.Logger) *InsightsIntegration {
	return &InsightsIntegration{
		Manager:         mgr,
		Log:             log,
		opName:          operatorName,
		opNamespace:     operatorNamespace,
		userAgentPrefix: userAgentPrefix,
		OSUtils:         &common.DefaultOSUtils{},
	}
}

var (
	ErrNoOperatorNamespace error = errors.New("operator namespace not detected")
	ErrNoOperatorName      error = errors.New("operator name not detected")
	ErrNoUserAgent         error = errors.New("user agent prefix not detected")
	ErrInsightsDisabled    error = errors.New("no Insights URL is available")
)

// Setup adds a controller to your manager, which creates and
// manages the HTTP proxy container that workloads may use
// to send reports to Red Hat Insights.
func (i *InsightsIntegration) Setup() error {
	if !i.isInsightsEnabled() {
		err := i.destroy()
		if err != nil {
			i.Log.Error(err, "failed to clean up Insights integration")
			return err
		}
		i.Log.Info("Insights integration is disabled")
		return err
	}

	// This will happen when attempting to run the operator locally
	if len(i.opNamespace) == 0 {
		return ErrNoOperatorNamespace
	}
	if len(i.opName) == 0 {
		return ErrNoOperatorName
	}
	if len(i.userAgentPrefix) == 0 {
		return ErrNoUserAgent
	}

	// Create the controller and add it to the manager
	ctx := context.Background()
	err := i.createInsightsController()
	if err != nil {
		i.Log.Error(err, "unable to add controller to manager", "controller", "Insights")
		return err
	}
	// Create a Config Map to be used as a parent of all Insights Proxy related objects
	err = i.createConfigMap(ctx)
	if err != nil {
		i.Log.Error(err, "failed to create config map for Insights")
		return err
	}

	// Compute the URL to the proxy server service
	insightsURL, err := i.GetProxyURL()
	if err != nil {
		return err
	}

	// Create the mutating webhook for injecting the agent
	err = i.createInsightsWebhook(insightsURL)
	if err != nil {
		i.Log.Error(err, "unable to create webhook", "webhook", "Pod")
		return err
	}

	i.Log.Info("Insights proxy set up", "url", insightsURL.String())
	return nil
}

// destroy cleans up any existing proxy and any related objects
// it uses.
func (i *InsightsIntegration) destroy() error {
	// Delete any previously created Config Map (and its children)
	err := i.deleteConfigMap(context.Background())
	if err != nil {
		i.Log.Error(err, "failed to delete config map for Insights")
		return err
	}
	return nil
}

func (i *InsightsIntegration) isInsightsEnabled() bool {
	return strings.ToLower(i.GetEnv(common.EnvInsightsEnabled)) == "true"
}

func (i *InsightsIntegration) createInsightsController() error {
	config := &controller.InsightsReconcilerConfig{
		Client:          i.Manager.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("Insights"),
		Scheme:          i.Manager.GetScheme(),
		Namespace:       i.opNamespace,
		UserAgentPrefix: i.userAgentPrefix,
		OperatorName:    i.opName,
		OSUtils:         i.OSUtils,
	}
	controller, err := controller.NewInsightsReconciler(config)
	if err != nil {
		return err
	}
	if err := controller.SetupWithManager(i.Manager); err != nil {
		return err
	}
	return nil
}

func (i *InsightsIntegration) createInsightsWebhook(insightsURL *url.URL) error {
	agentWebhook := webhooks.NewAgentWebhook(&webhooks.AgentWebhookConfig{
		InsightsURL: insightsURL,
	})
	if err := agentWebhook.SetupWebhookWithManager(i.Manager); err != nil {
		return err
	}
	return nil
}

func (i *InsightsIntegration) createConfigMap(ctx context.Context) error {
	// The config map should be owned by the operator deployment to ensure it and its descendants are garbage collected
	owner := &appsv1.Deployment{}
	// Use the APIReader instead of the cache, since the cache may not be synced yet
	err := i.Manager.GetAPIReader().Get(ctx, types.NamespacedName{
		Name: i.opName, Namespace: i.opNamespace}, owner)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.InsightsConfigMapName(i.opName),
			Namespace: i.opNamespace,
		},
	}
	err = controllerutil.SetControllerReference(owner, cm, i.Manager.GetScheme())
	if err != nil {
		return err
	}

	err = i.Manager.GetClient().Create(ctx, cm, &client.CreateOptions{})
	if err == nil {
		i.Log.Info("Config Map for Insights created", "name", cm.Name, "namespace", cm.Namespace)
	}
	// This may already exist if the pod restarted
	return client.IgnoreAlreadyExists(err)
}

func (i *InsightsIntegration) deleteConfigMap(ctx context.Context) error {
	// Children will be garbage collected
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.InsightsConfigMapName(i.opName),
			Namespace: i.opNamespace,
		},
	}

	err := i.Manager.GetClient().Delete(ctx, cm, &client.DeleteOptions{})
	if err == nil {
		i.Log.Info("Config Map for Insights deleted", "name", cm.Name, "namespace", cm.Namespace)
	}
	// This may not exist if no config map was previously created
	return client.IgnoreNotFound(err)
}

func (i *InsightsIntegration) GetProxyURL() (*url.URL, error) {
	if !i.isInsightsEnabled() {
		return nil, ErrInsightsDisabled
	}
	return &url.URL{
		Scheme: "http", // TODO add https support
		Host: fmt.Sprintf("%s.%s.svc:%d", common.ProxyServiceName(i.opName), i.opNamespace,
			common.ProxyServicePort),
	}, nil
}
