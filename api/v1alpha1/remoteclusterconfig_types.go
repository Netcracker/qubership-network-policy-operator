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

// RemoteCluster defines remote cluster address and credentials to connect to
type RemoteCluster struct {
	Name   string `json:"name"`
	API    string `json:"api"`
	Secret string `json:"remoteSecret"`
}

// LocalCluster is needed to indicate the local cluster resources on remote cluster
type LocalCluster struct {
	Name string `json:"name"`
	API  string `json:"api"`
}

// RemoteClusterConfigSpec defines the desired state of RemoteClusterConfig
type RemoteClusterConfigSpec struct {
	RemoteCluster RemoteCluster `json:"remoteCluster"`
	LocalCluster  LocalCluster  `json:"localCluster"`
}

// RemoteClusterConfigStatus defines the observed state of RemoteClusterConfig
type RemoteClusterConfigStatus struct {
	Conditions           []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	ServiceSubnet        string             `json:"serviceSubnet,omitempty"`
	CoupleServiceSubnet  string             `json:"coupleServiceSubnet,omitempty"`
	KubeAPIService       string             `json:"kubeAPIService,omitempty"`
	CoupleKubeAPIService string             `json:"coupleKubeAPIService,omitempty"`
	CoreDNSService       string             `json:"coreDNSService,omitempty"`
	CoupleCoreDNSService string             `json:"coupleCoreDNSService,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RemoteClusterConfig is the Schema for the remoteclusterconfigs API
type RemoteClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteClusterConfigSpec   `json:"spec,omitempty"`
	Status RemoteClusterConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RemoteClusterConfigList contains a list of RemoteClusterConfig
type RemoteClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteClusterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteClusterConfig{}, &RemoteClusterConfigList{})
}
