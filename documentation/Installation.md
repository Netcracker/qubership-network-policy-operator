This manual provides information on installation procedure of Network Policy Operator.

## Prerequisites

The prerequisites for the installation are as follwos:

- kubectl version v1.11.3+
- Calico CNI with API server version v3.28.1+
- Kubernetes v1.27+ or OCP4.14+ cluster
- Cluster admin access to the cluster
- In case of DR, make sure that you have configured DR clusters correctly with cross-cluster Calico CNI pod-to-pod/pod-to-service connectivity and cross-cluster CoreDNS service names resolving 

Allow IPIP traffic between nodes on IaaS layer. Set `ipipMode: Always` in pod network `IPPool`, for example:

```yaml
apiVersion: projectcalico.org/v3
kind: IPPool
metadata:
  name: default-ipv4-ippool
spec:
  allowedUses:
  - Workload
  - Tunnel
  blockSize: 26
  cidr: 10.128.0.0/14
  ipipMode: Always
  natOutgoing: true
  nodeSelector: all()
  vxlanMode: Never
```

## Deployment with Helm Chart

The deployment with Helm Chart is described below.

### Parameters

The parameters are specified in the below table.

| Parameter                           |	Type   |	Default value                                                                                                                           |	Description                                                                                      |
|-------------------------------------|--------|--------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|
| nameOverride                        | string | ""                                                                                                                                         | Override chart name                                                                                |
| fullnameOverride                    | string | ""                                                                                                                                         | Override FQDN                                                                                      |
| replicaCount                        | int    | 1                                                                                                                                          | Number of operator replicas                                                                        |
| podAnnotations                      | object | {}                                                                                                                                         | Custom operator Pod annotations                                                                    |
| podLabels                           | object | {}                                                                                                                                         | Custom operator Pod labels                                                                         |
| podSecurityContext                  | object | {"runAsNonRoot": true}                                                                                                                     | Pod level Security Context                                                                         |
| serviceAccount.create               | bool   | true                                                                                                                                       | If true Service Account will be created                                                            |
| serviceAccount.automount            | bool   | true                                                                                                                                       | Automount API credentials for the Service Account                                                  |
| serviceAccount.annotations          | object | {}                                                                                                                                         | Custom Service Account annotations                                                                 |
| serviceAccount.name                 | string | "network-policy-operator"                                                                                                                  | Service Account name                                                                               |
| leaderElection.role                 | string | leader-election-role                                                                                                                       | Name of the Leader election Role                                                                   |
| leaderElection.rolebinding          | string | leader-election-rolebinding                                                                                                                | Name of the Leader election Rolebinding                                                            |
| manager.image.repository            | string | ghcr.io/netcracker/qubership-network-policy-operator                                                                                       | Registry of manager container image                                                                |
| manager.image.pullPolicy            | string | IfNotPresent                                                                                                                               | Pulling policy of manager container image                                                          |
| manager.image.tag                   | string | ""                                                                                                                                         | Manager container image tag                                                                        |
| manager.imagePullSecrets            | array  | []                                                                                                                                         | Secret to connect to the registry                                                                  |
| manager.rbac.create                 | bool   | true                                                                                                                                       | If true Cluster Role and Rolebindigs will be created for operator                                  |
| manager.rbac.role                   | string | network-policy-operator-role                                                                                                               | Name of the operator Cluster Role                                                                  |
| manager.rbac.rolebinding            | string | network-policy-operator-rolebinding                                                                                                        | Name of the operator Cluster Rolebinding                                                           |
| manager.securityContext             | object | {"runAsNonRoot": true, "allowPrivilegeEscalation": false, "capabilities": {"drop": ["ALL"]}, "seccompProfile": {"type": "RuntimeDefault"}} | Manager container Security Context                                                                 |
| manager.resources                   | object | {"limits": {"cpu": "500m", "memory": "512Mi"}, "requests": {"cpu": "100m", "memory": "128Mi"}}                                             | Manager container CPU/memory resource                                                              |
| manager.livenessProbe               | object | {"httpGet": {"path": "/healthz", "port": 8081}, "initialDelaySeconds": 15, "periodSeconds": 20}                                            | Manager container liveness probe                                                                   |
| manager.readinessProbe              | object | {"httpGet": {"path": "/readyz", "port": 8081}, "initialDelaySeconds": 5, "periodSeconds": 10}                                              | Manager container readiness probe                                                                  |
| manager.volumes                     | array  | []                                                                                                                                         | Volumes to add to the pod                                                                          |
| manager.volumeMounts                | array  | []                                                                                                                                         | Volumes mounts                                                                                     |
| authProxy.image.repository          | string | gcr.io/kubebuilder/kube-rbac-proxy                                                                                                         | Registry of Auth proxy container image                                                             |
| authProxy.image.pullPolicy          | string | IfNotPresent                                                                                                                               | Pulling policy of Auth proxy container image                                                       |
| authProxy.image.tag                 | string | "v0.15.0"                                                                                                                                  | Auth proxy container image tag                                                                     |
| authProxy.imagePullSecrets          | array  | []                                                                                                                                         | Secret to connect to the registry                                                                  |
| authProxy.securityContext           | object | {"runAsNonRoot": true, "allowPrivilegeEscalation": false, "capabilities": {"drop": ["ALL"]}, "seccompProfile": {"type": "RuntimeDefault"}} | Auth proxy container Security Context                                                              |
| authProxy.resources                 | object | {"limits": {"cpu": "500m", "memory": "512Mi"}, "requests": {"cpu": "50m", "memory": "64Mi"}}                                               | Auth proxy container CPU/memory resource                                                           |
| authProxy.rbac.create               | bool   | true                                                                                                                                       | If true Cluster Role and Rolebindigs will be created for Auth proxy                                |
| authProxy.rbac.role                 | string | network-policy-operator-proxy-role                                                                                                         | Name of the Auth proxy Cluster Role                                                                |
| authProxy.rbac.rolebinding          | string | network-policy-operator-proxy-rolebinding                                                                                                  | Name of the Auth proxy Cluster Rolebinding                                                         |
| authProxy.rbac.metrics              | string | metrics-reader                                                                                                                             | Name of the Auth proxy client Cluster Role                                                         |
| nodeSelector                        | object | {}                                                                                                                                         | Node labels for pod assignment                                                                     |
| tolerations                         | array  | []                                                                                                                                         | Node tolerations for pod assignment                                                                |
| affinity                            | object | {}                                                                                                                                         | Node affinity for pod assignment                                                                   |
| networkPolicies.name                | string | "default"                                                                                                                                  | NetworkPolicyConfig name                                                                           |
| networkPolicies.enabled             | bool   | true                                                                                                                                       | If true NetworkPolicies will be created                                                            |
| networkPolicies.scope               | object | {}                                                                                                                                         | Namespace label where NetworkPolicies must be created, if empty all of the namespaces are included |
| networkPolicies.allowAnnotation     | string | "network-policy.qubership.org/allowed-from"                                                                                                | Annotation key to identify the allowed namespaces                                                  |
| networkPolicies.controlPlane        | string | "kube-,openshift,calico,tigera,ingress-nginx"                                                                                              | Comma-separated list of namespaces that must not be restricted, processed as `start with`          |
| networkPolicies.implicitNamespaces  | string | "openshift-monitoring,platform-monitoring,ingress-nginx,openshift-ingress"                                                                 | Comma-separated list of namespaces that are allowed by default                                     |
| networkPolicies.customPolicies      | string | ""                                                                                                                                         | Custom NetworkPolicies in raw YAML (only Calico notation)                                          |
| networkPolicies.logs.enabled        | bool   | false                                                                                                                                      | If true the `Log` action will be added into NetworkPolicies                                        |
| networkPolicies.logs.collectingAll  | bool   | true                                                                                                                                       | If true the all of the connections are logged, if false only denied                                |
| networkPolicies.logs.namespaceScope | bool   | "network-policy.qubership.org/collecting-logs"                                                                                             | Namespace label key to disable logs flexible                                                       |
| networkPolicies.defaultTier         | string | automated                                                                                                                                  | OOB NetworkPolicies Tier                                                                           |
| networkPolicies.customTier          | string | default                                                                                                                                    | Custom NetworkPolicies Tier                                                                        |
| dr.install                          | bool   | false                                                                                                                                      | If true RemoteClusterConfig will be created                                                        |
| dr.localCluster                     | object | {}                                                                                                                                         | Local cluster configuration                                                                        |
| dr.remoteClusters                   | array  | []                                                                                                                                         | List of remote cluster configuration                                                               |
| dr.authentication                   | string | token                                                                                                                                      | Type of authentication in DR scheme                                                                |
| dr.tls                              | string | auto                                                                                                                                       | Automation or manual TLS certificate creation                                                      |
| dr.tlsJob.image                     | object | {"repository": "ghcr.io/netcracker/qubership-network-policy-job","pullPolicy": "IfNotPresent"}                                             | TLS cetrtificate creation Job specification                                                        |
| dr.tlsJob.resources                 | object | {"requests": {"cpu": "100m","memory": "128Mi"}}                                                                                            | TLS cetrtificate creation Job specification                                                        |
| dr.tlsJob.nodeSelector              | object | {"kubernetes.io/os": "linux"}                                                                                                              | TLS cetrtificate creation Job specification                                                        |
| dr.tlsJob.tolerations               | array  | []                                                                                                                                         | TLS cetrtificate creation Job specification                                                        |
| dr.tlsJob.securityContext           | object | {"runAsNonRoot": true}                                                                                                                     | TLS cetrtificate creation Job specification                                                        |

### Basic Installation

The installation procedure must be performed on each of the clusters in case of DR scheme.

**Namespace Creation**

```sh
kubectl create ns netpolconf-system
```

**values.yaml Configuration**

Navigate to the `chart/network-policy-operator/` directory and edit `values.yaml` according to the environment configuration. In case of DR scheme, it is better to use the unified secret naming, that means the secrets with token to connect to the same remote cluster have the same names on different clusters. The clusters names must be always be unique within the set of cluters. There are several options that must be changed in `values.yaml` to get the properly working cluster after NetworkPolicies application. The `networkPolicies` section includes several options that could be configured.

* `controlPlane` sets the namespaces prefixes for the namespaces that must not be restricted by NetworkPolicies. That means, any incoming connections to the pods in that namespaces are allowed.
* `allowAnnotation` annotation must be set on each namespace preliminary according to solution internal interaction.
* All of the namespaces that need unconditional access to any other namespace such as monitoring, ingress controller must be added to `implicitNamespaces`.
* The processed namespaces could be limited by label that specified in `scope` option. That could be useful in multi-vender environmens or during testing and adjustment. Part of `values.yaml` example:

```
...
networkPolicies:
  name: "default"
  enabled: true
  scope:
    my-label: oob-network-policy
...
```
* `customPolicies` option includes custom Calico NetworkPolicies in YAML syntax. These polilies are being applied as-is.
* `enabled` option switches on and off NetworkPolicies globaly

The `logs` section manages the connections logs. There are several options to configure it:

* If `collectingAll` is `true`, the all of the connections are logged. If it is `false`, only denied connections are logged.
* `namespaceScope` defines the label that must be set in `false` or `none` to exclude the labeled namespace connections from logs.
* `enabled` option enables and disables the connections logs globally within the cluster.

Calico v3.29 and higher needs to set the NetwokPolicies `Tiers`. The default values are the following:

```
...
networkPolicies:
...
  defaultTier: automated
  customTier: default
...
```

* `defaultTier` sets the `Tier` that is used for all OOB NetworkPolicies
* `customTier` sets the custom NetworkPolicies `Tier`

The `defaultTier` is being created during the installation, whereas the `customTier` is the `Tier` that provided by Calico CNI itself. In case of change the `customTier` create the resource preliminary. Keep in mind the `customTier` order must be bigger than `defaultTier` order, which is `10000`. For more information, refer to the _Official Documentation_ at [https://docs.tigera.io/calico/latest/reference/resources/tier](https://docs.tigera.io/calico/latest/reference/resources/tier)

In case of DR scheme, check the [DR Preparation](#dr-preparation) section below.

**Installation**

```sh
helm install npc-operator . -n netpolconf-system
```

### DR Preparation

The DR preparation includes two steps. The first one must be performed before installation and the second one after the installation.

#### Pre-installation

The `dr` section includes options to configure the interation between clusters in DR scheme.

* `install` option enables/disables `RemoteClusterConfig` creation
* `localCluster` describes local cluster settings
* `remoteClusters` describes list of remote clusters settings

Cluster settings:

* `name` option is a cluster name, it must be uniq in clusters list
* `api` option is a particular Kubernetes API endopint, FQDN and port 
* `secret` option is secret to connect to the particular cluster, the secret located in operator namespace
* `authentication` defines the authentication type; `token` is default; `tls` means that operators will use TLS certificates to connect to other clusters

#### Post-installation

The post-installation process is described below.

**Token authentication**

Change token inside the `Secret` to connect to remote cluster in the following maner:

Run on cluster that is `localCluster` for one of the `values.yaml` files

```sh
kubectl -n netpolconf-system get secret <NAME OF THE SECRET FROM `dr.localCluster.secret` OPTION IN values.yaml> -o=jsonpath='{.data.token}'
```

Take the output of previous command and run the command on the other clusters (do not forget to change kubeconfig) 

```sh
kubectl -n netpolconf-system patch secret <THE NAME OF THE SECRET THAT IS USED TO CONNECT FROM CURRENT CLUSTER TO THE CLUSTER ABOVE>  --type json -p '[{"op": "replace", "path": "/data/token", "value": "<PREVIOUS COMMAND OUTPUT>"}]'
```

Do the same procedure with other clusters in DR scheme.

**TLS authentication**

Save the TLS certificate `Secret` into file:

```sh
kubectl -n netpolconf-system get secret <NAME OF THE SECRET FROM `dr.localCluster.secret` OPTION IN values.yaml> -o yaml > secret.yaml
```

Apply it on the clusters in DR scheme:

```sh
kubectl apply -f secret.yaml
```

Repeat the procedure for all of the clusters in DR scheme.

### Check installation

The the successfull instalation could be checked by `NetworkPolicyConfig` custom resource:

```sh
kubectl get networkpolicyconfig
```

The ouput must include only one resource with name from `values.yaml`.

Also, the `NetworkPolicies` could be created on cluster (it depends on configuration). It could be check by the command:

```sh
kubectl get cnp -A
```

The output is similar to the following:

```
NAMESPACE           NAME              CREATED AT
default             allow-from-ns     2024-10-15T11:15:44Z
default             allow-within-ns   2024-10-15T11:15:44Z
default             default-deny      2024-10-15T11:15:44Z
test                allow-from-ns     2024-10-15T11:43:08Z
test                allow-within-ns   2024-10-15T11:43:08Z
test                default-deny      2024-10-15T11:43:08Z
```

In case of Calico v3.29+ check the `Tiers`:

```sh
kubectl get tiers
```

The output is similar to the following:

```
NAME                 CREATED AT
adminnetworkpolicy   2024-12-06T10:47:17Z
automated            2025-04-08T13:07:32Z
default              2024-12-06T10:47:15Z
```

You can check the successfull instalation in DR scheme by `RemoteClusterConfig` custom resource:

```sh
kubectl -n netpolconf-system get remoteclusterconfig
```

In case of correct DR settings, the `RemoteClusterConfig` must have at least the following message in status:

```
status:
  conditions:
  - lastTransitionTime: "2024-10-10T11:02:49Z"
    message: Couple is identified
    reason: Initializing
    status: "True"
    type: Coupled
  serviceSubnet  = "172.30.0.0/16"
  kubeAPIService = "172.30.0.1"
  coreDNSService = "172.30.0.10"
  coupleServiceSubnet  = "172.31.0.0/16"
  coupleKubeAPIService = "172.31.0.1"
  coupleCoreDNSService = "172.31.0.10"
```
