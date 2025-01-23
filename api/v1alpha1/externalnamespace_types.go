/*
Copyright 2024.

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

// Resource defines resource specification
type Resources struct {
	Pods	 []Pod	   `json:"pods,omitempty"`
	Services []Service `json:"services,omitempty"`
}

// Pod defines the most important options of regular Pod
type Pod struct {
	Labels   map[string]string `json:"labels,omitempty"`
	Name     string            `json:"name"`
	NodeName string            `json:"nodeName,omitempty"`
	HostIP   string            `json:"hostIP,omitempty"`
	PodIP    string            `json:"podIP,omitempty"`
}

// Service defines the most important options of regular Service
type Service struct {
	Selector  map[string]string `json:"selectors,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Name	  string            `json:"name"`
	ClusterIP string            `json:"clusterIP,omitempty"`
}

// ExternalNamespaceSpec defines the desired state of ExternalNamespace
type ExternalNamespaceSpec struct {
	RemoteCluster   string     `json:"remoteCluster"`
	RemoteNamespace string     `json:"remoteNamespace"`
	Resources       Resources  `json:"resources,omitempty"`
}

// ExternalNamespaceStatus defines the observed state of ExternalNamespace
type ExternalNamespaceStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ExternalNamespace is the Schema for the externalnamespaces API
type ExternalNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalNamespaceSpec   `json:"spec,omitempty"`
	Status ExternalNamespaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ExternalNamespaceList contains a list of ExternalNamespace
type ExternalNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalNamespace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalNamespace{}, &ExternalNamespaceList{})
}
