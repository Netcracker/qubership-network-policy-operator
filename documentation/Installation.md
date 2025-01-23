## Prerequisites
- kubectl version v1.11.3+
- Calico CNI with API server version v3.28.1+
- Kubernetes v1.27+ or OCP4.14+ cluster
- Cluster admin access to the cluster
- In case of DR, the following scheme must be configured: [DR Scheme](https://github.com/Netcracker/DRNavigator/tree/main/documentation/DRScheme.md)

Allow IPIP traffic between nodes on IaaS layer. Set `ipipMode: Always` in pod network `IPPool` e.g.:

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

### Basic Installation

The installation procedure must be performed on each of the clusters in case of DR scheme

**Namespace Creation**

```sh
kubectl create ns netpolconf-system
```

**values.yaml Configuration**

Navigate to `chart/network-policy-operator/` directory and edit `values.yaml` according to environment configuration. In case of DR scheme it's better to use unified secret naming, that means the secrets with token to connect to the same remote cluster have the same names on different clusters. The clusters names must be always uniq within the set of cluters. There are several options that must be changed in `values.yaml` to get the properly working cluster after NetworkPolicies application. The `networkPolicies` section includes several options that could be configured.

* `controlPlane` sets the namespaces prefixes for namespaces that must not be restricted by NetworkPolicies. That means any incoming connections to the pods in that namespaces are allowed
* `allowAnnotation` annotation must be set on each namespace preliminary according to solution internal interaction
* All of the namespaces that need unconditional access to any other namespace (e.g. monitoring, ingress controller) must be added to `implicitNamespaces`
* The processed namespaces could be limited by label that specified in `scope` option. That could be useful in multi-vender environmens or during testing and adjustment. part of `values.yaml` example:

```
...
networkPolicies:
  name: "default"
  enabled: true
  scope:
    my-label: oob-network-policy
...
```
* `customPolicies` option includes custom Calico NetworkPolicies in YAML syntax. These polilies are being applied as-is
* `enabled` option switches on and off NetworkPolicies globaly

The `logs` section manages the connections logs. There are several options to configure it:

* if `collectingAll` is `true` the all of the connections are logged, if it `false` only denied connections arelogged
* `namespaceScope` defines the label that must be set in `false` or `none` to exclude the labeled namespace connections from logs
* `enabled` option enables and disables the connections logs globally within the cluster

In case of DR scheme check the [DR preparation](#dr-preparation) article below

**Installation**

```sh
helm install npc-operator . -n netpolconf-system
```

### DR preparation

The DR preparation includes to steps. The first one must be performed before installation and the second one after the installation.

**Pre-installation**

The `dr` section includes options to configure interation between clusters in DR scheme

* `install` option enables/disables `RemoteClusterConfig` creation
* `localCluster` describes local cluster settings
* `remoteClusters` describes list of remote clusters settings

Cluster settings:

* `name` option is a cluster name, it must be uniq in clusters list
* `api` option is a particular Kubernetes API endopint, FQDN and port 
* `secret` option is secret to connect to the particular cluster, the secret located in operator namespace

**Post-installation**

Change token inside `Secret` to connect to remote cluster in the following maner:

Run on cluster that is `localCluster` for one of the `values.yaml` files

```sh
kubectl -n netpolconf-system get secret <NAME OF THE SECRET FROM `dr.localCluster.secret` OPTION IN values.yaml> -o=jsonpath='{.data.token}'
```

Take the output of previous command and run the command on the other clusters (do not forget to change kubeconfig) 

```sh
kubectl -n netpolconf-system patch secret <THE NAME OF THE SECRET THAT IS USED TO CONNECT FROM CURRENT CLUSTER TO THE CLUSTER ABOVE>  --type json -p '[{"op": "replace", "path": "/data/token", "value": "<PREVIOUS COMMAND OUTPUT>"}]'
```

### Check installation

The the successfull instalation could be checked by `NetworkPolicyConfig` custom resource:

```sh
kubectl get networkpolicyconfig
```

The ouput must include only one resource with name from `values.yaml`

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


The the successfull instalation in DR scheme could be checked by `RemoteClusterConfig` custom resource:

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
