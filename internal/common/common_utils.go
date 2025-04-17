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
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OSUtils is an abstraction on functionality that interacts with the operating system
type OSUtils interface {
	GetEnv(name string) string
}

type DefaultOSUtils struct{}

// GetEnv returns the value of the environment variable with the provided name. If no such
// variable exists, the empty string is returned.
func (o *DefaultOSUtils) GetEnv(name string) string {
	return os.Getenv(name)
}

// MergeLabelsAndAnnotations copies labels and annotations from a source
// to the destination ObjectMeta, overwriting any existing labels and
// annotations of the same key.
func MergeLabelsAndAnnotations(dest *metav1.ObjectMeta, srcLabels, srcAnnotations map[string]string) {
	// Check and create labels/annotations map if absent
	if dest.Labels == nil {
		dest.Labels = map[string]string{}
	}
	if dest.Annotations == nil {
		dest.Annotations = map[string]string{}
	}

	// Merge labels and annotations, preferring those in the source
	for k, v := range srcLabels {
		dest.Labels[k] = v
	}
	for k, v := range srcAnnotations {
		dest.Annotations[k] = v
	}
}

// NamespaceUniqueName appends a hash of the provided suffix to the name.
func NamespaceUniqueName(name string, suffixToHash string) string {
	// Use the 128-bit FNV-1 checksum of the suffix.
	hash := fnv.New128()
	hash.Write([]byte(suffixToHash))
	return fmt.Sprintf("%s-%x", name, hash.Sum([]byte{}))
}

const annotationSecretHash = "com.redhat.insights.runtimes/secret-hash"

func AnnotateWithSecretHash(ctx context.Context, client client.Client, namespace string, names []string,
	annotations map[string]string) (map[string]string, error) {
	if annotations == nil {
		annotations = map[string]string{}
	}
	result, err := hashSecrets(ctx, client, namespace, names)
	if err != nil {
		return nil, err
	}
	annotations[annotationSecretHash] = *result
	return annotations, nil
}

func hashSecrets(ctx context.Context, client client.Client, namespace string, names []string) (*string, error) {
	// Collect the JSON of all secret data, sorted by object name
	combinedJSON := []byte{}
	slices.Sort(names)
	for _, name := range names {
		secret := &corev1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, secret)
		if err != nil {
			return nil, err
		}
		// Marshal secret data as JSON. Keys are sorted, see: [json.Marshal]
		buf, err := json.Marshal(secret.Data)
		if err != nil {
			return nil, err
		}
		combinedJSON = append(combinedJSON, buf...)
	}
	// Hash the JSON with SHA256
	hashed := fmt.Sprintf("%x", sha256.Sum256(combinedJSON))
	return &hashed, nil
}

// SeccompProfile returns a SeccompProfile for the restricted
// Pod Security Standard that, on OpenShift, is backwards-compatible
// with OpenShift < 4.11.
// TODO Remove once OpenShift < 4.11 support is dropped
func SeccompProfile(openshift bool) *corev1.SeccompProfile {
	// For backward-compatibility with OpenShift < 4.11,
	// leave the seccompProfile empty. In OpenShift >= 4.11,
	// the restricted-v2 SCC will populate it for us.
	if openshift {
		return nil
	}
	return &corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	}
}
