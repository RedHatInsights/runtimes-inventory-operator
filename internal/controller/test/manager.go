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
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type FakeManager struct {
	ctrl.Manager
	client client.Client
	scheme *runtime.Scheme
	logger *logr.Logger
}

var _ ctrl.Manager = &FakeManager{}

func NewFakeManager(client client.Client, scheme *runtime.Scheme, logger *logr.Logger) *FakeManager {
	return &FakeManager{
		client: client,
		scheme: scheme,
		logger: logger,
	}
}

func (m *FakeManager) GetCache() cache.Cache {
	return nil
}

func (m *FakeManager) GetClient() client.Client {
	return m.client
}

func (m *FakeManager) GetScheme() *runtime.Scheme {
	return m.scheme
}

func (m *FakeManager) GetAPIReader() client.Reader {
	// May need to change if not using a fake client
	return m.client
}

func (m *FakeManager) GetControllerOptions() config.Controller {
	return config.Controller{
		// controller-runtime keeps a global cache of names for created controllers,
		// and ensures they are unique. Override this for testing.
		SkipNameValidation: &[]bool{true}[0],
	}
}

func (m *FakeManager) GetLogger() logr.Logger {
	return *m.logger
}

func (m *FakeManager) SetFields(interface{}) error {
	return nil
}

func (m *FakeManager) Add(manager.Runnable) error {
	return nil
}
