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

package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	NetworkPolicyConfigStatus = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "network_policy_config_status", 
			Help: "NetworkPolicyConfig status, could be 'disabled'=0; 'enabled for all namespaces'=1; 'enabled for some scope of namespaces'=2; 'failed'=-1",
		},
	)

	ConnectionsLogStatus = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "connections_log_status", 
			Help: "Connections audit log status, could be 'disabled'=-1; 'enabled for all connections'=0; 'enabled for denied connections only'=1",
		},
	)

	RemoteClustersConfigTotal = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_cluster_config_total", 
			Help: "Total number of RemoteClustersConfigs",
		},
	)

	RemoteClustersConfigLoose = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_cluster_config_loose", 
			Help: "Number of loose RemoteClustersConfigs",
		},
	)

	RemoteClustersConfigCoupled = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_cluster_config_coupled", 
			Help: "Number of coupled RemoteClustersConfigs",
		},
	)

	RemoteClustersConfigSynchronized = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_cluster_config_synchronized", 
			Help: "Number of synchronized RemoteClustersConfigs",
		},
	)

	NamespacesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "namespaces_total", 
			Help: "Total number of processed Namespaces",
		},
	)

	ExternalNamespacesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "external_namespaces_total", 
			Help: "Total number of ExternalNamespaces",
		},
	)

	NetworkPoliciesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "network_policies_total", 
			Help: "Total number of created NetworkPolicies",
		},
	)
)

// RegisterMetrics registers metrics with the global prometheus registry
func RegisterMetrics() {
	metrics.Registry.MustRegister(NetworkPolicyConfigStatus)
	metrics.Registry.MustRegister(ConnectionsLogStatus)
	metrics.Registry.MustRegister(RemoteClustersConfigTotal)
	metrics.Registry.MustRegister(RemoteClustersConfigLoose)
	metrics.Registry.MustRegister(RemoteClustersConfigCoupled)
	metrics.Registry.MustRegister(RemoteClustersConfigSynchronized)
	metrics.Registry.MustRegister(NamespacesTotal)
	metrics.Registry.MustRegister(ExternalNamespacesTotal)
	metrics.Registry.MustRegister(NetworkPoliciesTotal) 
}
