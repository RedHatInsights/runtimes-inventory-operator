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

package controller

import (
	"context"
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/RedHatInsights/runtimes-inventory-operator/internal/common"
	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// InsightsReconciler reconciles the Insights proxy for Runtimes agents
type InsightsReconciler struct {
	*InsightsReconcilerConfig
	backendDomain string
	proxyDomain   string
	proxyImageTag string
}

// InsightsReconcilerConfig contains configuration to create an InsightsReconciler
type InsightsReconcilerConfig struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	Namespace       string
	UserAgentPrefix string
	OperatorName    string
	common.OSUtils
}

// NewInsightsReconciler creates an InsightsReconciler using the provided configuration
func NewInsightsReconciler(config *InsightsReconcilerConfig) (*InsightsReconciler, error) {
	backendDomain := config.GetEnv(common.EnvInsightsBackendDomain)
	if len(backendDomain) == 0 {
		return nil, errors.New("no backend domain provided for Insights")
	}
	imageTag := config.GetEnv(common.EnvInsightsProxyImageTag)
	if len(imageTag) == 0 {
		return nil, errors.New("no proxy image tag provided for Insights")
	}
	proxyDomain := config.GetEnv(common.EnvInsightsProxyDomain)

	return &InsightsReconciler{
		InsightsReconcilerConfig: config,
		backendDomain:            backendDomain,
		proxyDomain:              proxyDomain,
		proxyImageTag:            imageTag,
	}, nil
}

// +kubebuilder:rbac:namespace=system,groups=apps,resources=deployments;deployments/finalizers,verbs=create;update;get;list;watch
// +kubebuilder:rbac:namespace=system,groups="",resources=services;secrets;configmaps/finalizers,verbs=create;update;get;list;watch
// +kubebuilder:rbac:namespace=system,groups="",resources=configmaps,verbs=create;update;delete;get;list;watch
// +kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;list;watch
// OLM doesn't let us specify RBAC for openshift-config namespace, so we need a cluster-wide permission
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch,resourceNames=pull-secret

// Reconcile processes the Insights proxy deployment and configures it accordingly
func (r *InsightsReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Insights Proxy")

	// Reconcile all Insights support
	err := r.reconcileInsights(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InsightsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("insights").
		// Filter controller to watch only specific objects we care about
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.isPullSecretOrProxyConfig)).
		Watches(&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.isProxyDeployment)).
		Watches(&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.isProxyService))
	return c.Complete(r)
}

func (r *InsightsReconciler) isPullSecretOrProxyConfig(ctx context.Context, secret client.Object) []reconcile.Request {
	if !(secret.GetNamespace() == "openshift-config" && secret.GetName() == "pull-secret") &&
		!(secret.GetNamespace() == r.Namespace && secret.GetName() == common.ProxySecretName(r.OperatorName)) {
		return nil
	}
	return r.proxyDeploymentRequest()
}

func (r *InsightsReconciler) isProxyDeployment(ctx context.Context, deploy client.Object) []reconcile.Request {
	if deploy.GetNamespace() != r.Namespace || deploy.GetName() != common.ProxyDeploymentName(r.OperatorName) {
		return nil
	}
	return r.proxyDeploymentRequest()
}

func (r *InsightsReconciler) isProxyService(ctx context.Context, svc client.Object) []reconcile.Request {
	if svc.GetNamespace() != r.Namespace || svc.GetName() != common.ProxyServiceName(r.OperatorName) {
		return nil
	}
	return r.proxyDeploymentRequest()
}

func (r *InsightsReconciler) proxyDeploymentRequest() []reconcile.Request {
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: r.Namespace, Name: common.ProxyDeploymentName(r.OperatorName)}}
	return []reconcile.Request{req}
}
