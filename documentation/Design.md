# Calico CNI Based OOB NetworkPolicies

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [Test Plan](#test-plan)
      - [Prerequisite testing updates](#prerequisite-testing-updates)
      - [Unit tests](#unit-tests)
      - [Integration tests](#integration-tests)
      - [e2e tests](#e2e-tests)
  - [Graduation Criteria](#graduation-criteria)
  - [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
  - [Version Skew Strategy](#version-skew-strategy)
- [Production Readiness Review Questionnaire](#production-readiness-review-questionnaire)
  - [Feature Enablement and Rollback](#feature-enablement-and-rollback)
  - [Rollout, Upgrade and Rollback Planning](#rollout-upgrade-and-rollback-planning)
  - [Monitoring Requirements](#monitoring-requirements)
  - [Dependencies](#dependencies)
  - [Scalability](#scalability)
  - [Troubleshooting](#troubleshooting)
- [Implementation History](#implementation-history)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)
<!-- /toc -->

## Summary

Platform solution that is based on Kubernetes or OpenShift cluster must have OOB NetworkPolicies due to security reasons. Basically, any traffic in cluster must be restricted and as the result only explicitly specified pods/namespaces have access to one another.

## Motivation

### Goals

* Restrict incoming traffic to particular namespace
* Restrict outgoing traffic to Kubernetes system services
* Set arbitrary NetworkPolicy
* Work in [DR Scheme](https://github.com/Netcracker/DRNavigator/tree/main/documentation/DRScheme.md)

### Non-Goals

* NetworkPolicy visualization 
* UI NetworkPolicy configuration
* Allowed/Denied connections logs

## Proposal

Since typical Kubernetes or OpenShift cluster works with Calico CNI, the solution should use Calico CNI functionality to cover current requirements. OOB NetworkPolicies are based on [Calico NetworkPolicy](https://docs.tigera.io/calico/latest/reference/resources/networkpolicy) specification and include the following parts:

* Namespaced NetworkPolicies. The ingress rules in that policies explicitly allow incoming traffic from other namespaces. Namespaces that are allowed to connect to particular namespace listed in special annotation. Traffic from monitoring namespace is allowed implicitly by default. The egress rule allowes all outgoing traffic from all of the pods in the namespace
* Kubernetes system namespaces such as 'kube-system', 'calico-apiserver' are not restricted by NetworkPolicies 
* Arbitrary NetworkPolicies. That part allows to create any policies for any services in the cluster
* Dynamic NetworkPolicies to cover DR scheme

All of the NetworkPolicies are managed by the Kubernetes operator. Operator is being installed by Helm Chart. Installation in DR scheme is provided with additional automated preinstallation procedure to swap the secrets and identify remote clusters

### Risks and Mitigations

The huge number of custom resource in Kubernetes cluster affects ETCD size, that leads to control-palne performnace decrease. The number of custom resources could be limited as the prerequisite to solution use case

The operator request/limits need to be assessed during special tests

## Design Details

### Namespaced NetworkPolicies

Namespaced NetworkPolicies example

```
---
apiVersion: projectcalico.org/v3
kind: NetworkPolicy
metadata:
  name: allow-within-ns
  namespace: default
spec:
  selector: all()
  types:
    - Ingress
  ingress:
    - action: Allow
      source:
        selector: all()
---
apiVersion: projectcalico.org/v3
kind: NetworkPolicy
metadata:
  name: allow-from-ns
  namespace: default
spec:
  selector: all()
  types:
    - Ingress
  ingress:
    - action: Allow
      protocol: TCP
      source:
        selector: all()
        namespaceSelector: kubernetes.io/metadata.name in {'frontend-1', 'frontend-2'}
      destination:
        ports: [ '1:65535']
---
apiVersion: projectcalico.org/v3
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: default
spec:
  selector: all()
  types:
    - Ingress
---
apiVersion: projectcalico.org/v3
kind: NetworkPolicy
metadata:
  name: restrict-service
spec:
  order: 10
  egress:
  - action: Allow
    destination:
      nets:
        - 0.0.0.0/0
  selector: all()
  types:
  - Egress
```

### Dynamic NetworkPolicies and Operator

![Operator scheme](/documentation/images/NP_Operator.png)

The main part of operator needs NetworkPolicyConfig custom resource that describes templates for NetworkPolicies above. It consists of the one controller that watches NetworkPolicyConfig and Namespaces. The controller creates or deletes NetworkPolicies in particular Namespace

`NetworkPolicyConfig` spec example:

```
apiVersion: netcracker.com/v1alpha1
kind: NetworkPolicyConfig
metadata:
  name: npc-example
spec:
  enabled: true
  scope: {}
  allowAnnotation: "network-policy.netcracker.com/allowed-from"
  controlPlane: "kube-,openshift,calico,tigera,ingress-nginx"
  implicitNamespaces: "openshift-monitoring,platform-monitoring,ingress-nginx,openshift-ingress" 
  policies:
    AllowFromNamespace: |
      apiVersion: projectcalico.org/v3
      kind: NetworkPolicy
      metadata:
        name: allow-from-ns
        namespace: default
      spec:
        selector: all()
        types:
          - Ingress
        ingress:
          - action: Allow
            source:
              namespaceSelector: all()
    AllowWithinNamespace: |
      apiVersion: projectcalico.org/v3
      kind: NetworkPolicy
      metadata:
        name: allow-within-ns
        namespace: default
      spec:
        selector: all()
        types:
          - Ingress
        ingress:
          - action: Allow
            source:
              selector: all()
    CustomPolicies: ""
    DefaultDeny: |
      apiVersion: projectcalico.org/v3
      kind: NetworkPolicy
      metadata:
        name: default-deny
        namespace: default
      spec:
        selector: all()
        types:
          - Ingress
    RestrictServices: |
      apiVersion: projectcalico.org/v3
      kind: NetworkPolicy
      metadata:
        name: restrict-service
      spec:
        order: 10
        egress:
        - action: Allow
          destination:
            nets:
              - 0.0.0.0/0
        selector: all()
        types:
        - Egress
```

For DR scheme the information about remote cluster pods must be presented and actualized on local cluster to create NetworkPolicy for that pods. The operator needs IP of a pod and pod labels. Pod labels are matched with the existing NetworkPolicy rules and as the result operator create/update a NetworkPolicy with the same logic as the exiting one but applicable for remote cluster pods because selectors are replaced by the exact pods IPs. For pod to service interactions should be covered by egress rules

The DR part of operator needs two custom resources:

* `RemoteClusterConfig` describes settings to connect to remote clusters
* `ExternalNamespace` describes pods specs for particular namespace from remote cluster

The DR part of operator consists of the following controllers:

* Controller #1 watches the Namespaces and creates or updates ExternalNamespace. It's needed to process a Namespace creation or Namespace description change
* Controller #2 watches the Pods and updates ExternalNamespace It's needed to process Pod creation or Pod description change
* Controller #3 watches the NetworkPolicies and creates or updates additional NetworkPolicy to cover the traffic for Pods from remote cluster
* Controller #4 watches RemoteClusterConfig and creates ExternalNamespace. It's needed for initial creation of ExternalNamespace
* Controller #5 watches the Service and updates ExternalNamespace. It's needed to process Service creation and Services description changes 

Let's call the NetworkPolicy that must be extended to cover remote cluster pods an `origin` policy. The Networkpolicy that will be created during reconciliation loop of operator is a `target` policy. During reconsiliation loop operator creates/updates/delete the ExternalNamespaces on remote cluster. It makes possible to use the ExternalNamespaces on remote cluster to create/update the `target` NetworkPolicy. The `target` policy is labeled to be indicated as `target` policy. The `RemoteClusterConfig` resources must be created on both clusters to start the pods specification swap. In DR scheme all of the labels in clusters are transparent for both sides, therefor traffic to/from namespaces in different clusters but with the same names is allowed by default. The pod to service connection NetworkPolicy works in a different maner. It explicitly allowes outgoing connections to particular services from allowed namespaces in neighbor cluster and denies connections to other service. The outgoing traffic to any endpoints out of the service subnet of neighbor cluster must be allowed

`RemoteClusterConfig` spec example:

```
apiVersion: netcracker.com/v1alpha1
kind: RemoteClusterConfig
metadata:
  name: test-remote-cluster-config
  namespace: operator-namespace
spec:
  localCluster:
    api: all.new.local:6443
    name: cluster-1
  remoteCluster:
    api: all.new.local:6443
    name: cluster-2
    remoteSecret: remote-cluster2
```

`ExternalNamespace` spec example:

```
apiVersion: netcracker.com/v1alpha1
kind: ExternalNamespace
metadata:
  labels:
    test-label: true
  name: ext-ns
  namespace: operator-namespace
spec:
  remoteCluster: cluster-2
  remoteNamespace: test
  resources:
    pods:
    - hostIP: 192.168.56.106
      labels:
        app: target
        test-label: test-value
      name: test-pod-1
      nodeName: ubuntu-1
      podIP: 10.128.128.128
    - hostIP: 192.168.56.106
      labels:
        app: target
        test-label: test-value
      name: test-pod-2
      nodeName: ubuntu-2
      podIP: 10.128.128.129
    services:
	  selectors: 
        app: target
      labels: {}
      name: test-svc
      clusterIP: 172.30.10.10
```

### Helm Chart

The `values.yaml` draft is the following:

```
networkPolicies:
  install: true
  name: "default"
  scope: {} 
  allowAnnotation: "network-policy.netcracker.com/allowed-from"
  controlPlane: "kube-,openshift,calico,tigera,ingress-nginx"
  implicitNamespaces: "openshift-monitoring,platform-monitoring,ingress-nginx,openshift-ingress"
customPolicies:
  install: true
  spec: |
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: allow-to-mz
      namespace: dmz
    spec:
      selector: app.kubernetes.io/name in { 'public', 'egress' }
      egress:
        - action: Allow
          metadata:
            annotations:
              from: DMZ
              to: MZ
          destination:
            services:
              name: mz
              namespace: mz
dr:
  install: true
  cluster: cluster-1
  secret: cluster-1-secret
  remoteClusters:
    - name: cluster-2
      api: cluster-2.domain.com:6443
      secret: cluster-2-secret
```

Options description:

* `networkPolicies.install` enables/disables default NetworkPolicies
* `networkPolicies.scope` sets namespace label that identifies namespaces which must be protected by NetworkPolicies
* `networkPolicies.allowAnnotation` sets the annotation with coma separated namespaces list the traffic from where should be allowed. For instance, `network-policy.netcracker.com/allowed-from:app-1,app-2` annotation allows network access from `app-1`, `app-2` namespaces to annotated namespace
* `networkPolicies.controlPlane` sets the list of namespaces prefixes that are not restricted by NetworkPlolicies
* `networkPolicies.implicitNamespaces` sets namespaces from where outgoing traffic to any other namespaces is allowed by default. It's useful for monitoring
* `customPolicies` is a key-value map where arbitrary NetworkPolicies could be described
* `dr` section describes operator configuration
* `dr.cluster` is the current cluster uniq identifier
* `dr.remoteClusters` list consists of all of the remote clusters where the `ExternalNamespaces` will be manipulated
* `dr.remoteClusters[*].secret` is Secret with token to connect to particular remote cluster, it must have sufficient permissions to manipulate `ExternalNamespaces` on remote cluster

The preinstallation procedure gets list of `values.yaml` and `kubeconfig` for all of the clusters to prepare them

### Test Plan

##### Prerequisite testing updates

<!--
Based on reviewers feedback describe what additional tests need to be added prior
implementing this enhancement to ensure the enhancements have also solid foundations.
-->

##### Unit tests

Creation, update, and deletion operations for Namespace, Pod, ExternalNamespace, and RemoteClusterConfig

##### Integration tests

The same unit tests but for different Calico versions

##### e2e tests

Installation and upgrade OOB NetworkPolicicies on Kubernetes and OpenShift with and without operator

### Graduation Criteria

Iterations:

1. OOB NetworkPolicies for standlone cluster
2. Helm Chart for standalone cluster
3. Operator DR solution
4. Helm Chart for both stanalone cluster and DR solution

### Upgrade / Downgrade Strategy

Downgarade is prohibited

### Version Skew Strategy

Company bast practice

## Production Readiness Review Questionnaire

TODO

### Feature Enablement and Rollback

TODO

###### How can this feature be enabled / disabled in a live cluster?

Helm Chart installation / uninstallation

###### Does enabling the feature change any default behavior?

OOB NetworkPolicies affect only new network connections, the established connection are not affected

###### Can the feature be disabled once it has been enabled (i.e. can we roll back the enablement)?

Yes

###### What happens if we reenable the feature if it was previously rolled back?

The same as the enabling from the scratch

###### Are there any tests for feature enablement/disablement?

e2e tests

### Rollout, Upgrade and Rollback Planning

It's an independant feature

###### How can a rollout or rollback fail? Can it impact already running workloads?


###### What specific metrics should inform a rollback?

###### Were upgrade and rollback tested? Was the upgrade->downgrade->upgrade path tested?

###### Is the rollout accompanied by any deprecations and/or removals of features, APIs, fields of API types, flags, etc.?

### Monitoring Requirements

Monitoring metrics:

* OOB NetworkPolicies applied on cluster, namespace
* Operator is up and runnig
* Number of remote cluster
* Status of remote clusters
* Number of external namespaces


### Dependencies

###### Does this feature depend on any specific services running in the cluster?

Calico CNI, Calico APIServer:
* [Calico](https://docs.tigera.io/calico/latest/getting-started/)

### Scalability

TODO

###### Will enabling / using this feature result in any new API calls?

###### Will enabling / using this feature result in introducing new API types?

###### Will enabling / using this feature result in any new calls to the cloud provider?

###### Will enabling / using this feature result in increasing size or count of the existing API objects?

###### Will enabling / using this feature result in increasing time taken by any operations covered by existing SLIs/SLOs?

###### Will enabling / using this feature result in non-negligible increase of resource usage (CPU, RAM, disk, IO, ...) in any components?

###### Can enabling / using this feature result in resource exhaustion of some node resources (PIDs, sockets, inodes, etc.)?

### Troubleshooting

TODO

###### How does this feature react if the API server and/or etcd is unavailable?

###### What are other known failure modes?

###### What steps should be taken if SLOs are not being met to determine the problem?

## Implementation History

## Drawbacks

TODO

## Alternatives

There is no any alternatives for OOB NetworkPolicies

Operator for DR scheme could be replaced by:
[Calico Enterprise](https://docs.tigera.io/calico-enterprise/latest/about)

