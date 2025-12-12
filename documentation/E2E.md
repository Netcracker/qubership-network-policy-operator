## Run End to End Tests

### Stand-alone Tests

The deployment scheme is the following:

![Stand-alone](documentation/images/E2E_SA.png)

This part consists of the following steps:

1. Installation through Helm
2. Connectivity check between two namespaces (pod to pod and pod to service) when NetworkPolicyConfig is disabled
3. Connectivity check between two namespaces (pod to pod and pod to service) when NetworkPolicyConfig is enabled
4. Connectivity check between two namespaces (pod to pod and pod to service) when NetworkPolicyConfig is enabled and first namespace annotated to accept traffic from second namespace
5. Check that rejected connections are logged

Before run the tests it needs to set the following environment variables

```shell
export KUBECONFIG=/path/to/cluster.kubeconf
export CHART=./charts/network-policy-operator
export OP_IMG="ghcr.io/netcracker/qubership-network-policy-operator:main"
export NS="network-policy-operator"
export CURL_IMG="alpine/curl"
export CLUSTER="/path/to/KubeMarine/cluster.yaml"
export ECHO="./test/yamls/echo.yaml"
```

`KUBECONFIG` is kubeconf file where operator will be run
`CHART` is the path to Helm Chart
`OP_IMG` is the operator image
`NS` is the namespace where operator will be installed
`CURL_IMG` is the image with `curl`
`CLUSTER` is the path to cluster.yaml
`ECHO` is the path to manifest with echo service

### DR Tests

The deployment scheme is the following:

![DR](documentation/images/E2E_DR.png)

This part consists of the following steps:
1. Installation through Helm
2. Connection two clusters
3. Connectivity check between two namespaces from different clusters whith the same name (pod to pod and pod to service) when NetworkPolicyConfig is disabled 
4. Connectivity check between two namespaces from different clusters whith the different names (pod to pod and pod to service) when NetworkPolicyConfig is disabled
5. Connectivity check between two namespaces from different clusters whith the same name (pod to pod and pod to service) when NetworkPolicyConfig is enabled 
6. Connectivity check between two namespaces from different clusters whith the different names (pod to pod and pod to service) when NetworkPolicyConfig is enabled
7. Connectivity check between two namespaces from different clusters whith the same name (pod to pod and pod to service) when NetworkPolicyConfig is enabled and namespace that initiate connection is added to the list of implicit namespaces
8. Connectivity check between two namespaces from different clusters whith the different names (pod to pod and pod to service) when NetworkPolicyConfig is enabled and namespace that initiate connection is added to the list of implicit namespaces

Before run the tests it needs to set the following environment variables

```shell
export CHART=./charts/network-policy-operator
export OP_IMG="ghcr.io/netcracker/qubership-network-policy-operator:main"
export NS="network-policy-operator"
export CURL_IMG="alpine/curl"
export ECHO="./test/yamls/echo.yaml"
export KUBECONF1=/path/to/cluster-1.kubeconf
export KUBECONF2=/path/to/cluster-2.kubeconf
export VALUES1=/path/to/cluster-1/values.yaml
export VALUES2=/path/to/cluster-2/values.yaml
```
