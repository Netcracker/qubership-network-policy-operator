/*
Copyright NetCracker Technology Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LogsSpec struct {
	Enabled        bool   `json:"enabled,omitempty"`
	CollectingAll  bool   `json:"collectingAll,omitempty"`
	NamespaceLabel string `json:"namespaceScope,omitempty"`
}

type NamespacesRule struct {
	Namespace string   `json:"namespace"`
	AllowedTo []string `json:"allowedTo"`
}

// NetworkPolicyConfigSpec defines the desired state of NetworkPolicyConfig
type NetworkPolicyConfigSpec struct {
	Enabled            bool              `json:"enabled"`
	Scope              map[string]string `json:"scope,omitempty"`
	Policies           map[string]string `json:"policies,omitempty"`
	AllowAnnotation    string            `json:"allowAnnotation,omitempty"`
	ControlPlane       string            `json:"controlPlane,omitempty"`
	ImplicitNamespaces string            `json:"implicitNamespaces,omitempty"`
	Logs               LogsSpec          `json:"logs"`
	DefaultTier        string            `json:"defaultTier,omitempty"`
	CustomTier         string            `json:"customTier,omitempty"`
	NamespacesRules    []NamespacesRule  `json:"namespacesRules,omitempty"`
}

// NetworkPolicyConfigStatus defines the observed state of NetworkPolicyConfig
type NetworkPolicyConfigStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// NetworkPolicyConfig is the Schema for the networkpolicyconfig API
type NetworkPolicyConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkPolicyConfigSpec   `json:"spec,omitempty"`
	Status NetworkPolicyConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkPolicyConfigList contains a list of NetworkPolicyConfig
type NetworkPolicyConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkPolicyConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkPolicyConfig{}, &NetworkPolicyConfigList{})
}
