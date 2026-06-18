# Operations guide

This section provides information on the operations performed for the Network Policy Operator.

It consists of the following:
* Global Changes
* New Application Configuration
* Operator Check
* Audit Logs
* Upgrade with Helm Chart

## Global Changes

You can enable or disable the OOB NetworkPolicies globally by making changes in the NetworkPolicyConfig resource. Set the `spec.enabled` option to `true` to switch on OOB NetworkPolicies or set it to `false` to switch off OOB NetworkPolicies. The `spec.scope` option defines the label that could be set to namespace to reduce the scope of namespaces that are processed by network-policy-operator.

## New Application Configuration

Before the new application installation the following steps must be performed to get the working application:

* If the new application id a part of control-pale, add namespace into `spec.controlPlane`
* If the new application must have access to all of the other namespaces (for instance, to monitor some metrics) add namespace into `spec.implicitNamespaces`

Then create the rule in `namespacesRules` section in NetworkPolicyConfig for the application. Pay attention that notation works in different manner then the `network-policy.netcracker.com/allowed-from` annotation.

After that, check the list of NetworkPolicies in the application namespace and install the application if it is acceptable for a particular case.

## Operator Check

After successful installation/upgrade, the operator manipulates the NetworkPlolicies in each namespace. There is a simple way to check if the operator is working correctly. Run the following command and check the output:

```sh
kubectl -n <NAMESPACE> get cnp
```

The control-plane namespace doesn't have any policies, the others must have the list similar the following.

Standalone cluster:

```shell
NAME                          CREATED AT
allow-from-ns                 2024-11-29T09:18:35Z
allow-within-ns               2024-11-29T09:18:35Z
default-deny                  2024-11-29T09:18:36Z
```

DR clusters:

```shell
NAME                          CREATED AT
allow-from-ns                 2024-11-29T09:18:35Z
allow-within-ns               2024-11-29T09:18:35Z
default-deny                  2024-11-29T09:18:36Z
ens-netpol-obpn7djmzf-r9jn7   2024-11-29T09:18:37Z
restrict-service              2024-11-29T09:18:36Z
```

The incoming traffic policies:

* `allow-from-ns` defines the allowed Namespaces for current
* `allow-within-ns` allows traffic between pods in the same namespace
* policies with names starting with `ens-netpol` allows ingress traffic from neighbor cluster pods
* `default-deny` denies the rest of the ingress connections

The outgoing traffic policy:

* restrict-service restricts egress connections to the neighbor cluster services subnet

All of the NetworkPolicies that has been created by operator have `network-policy.netcracker.com/generated: "true"` label. `ens-netpol` NetworkPolicies have `network-policy.netcracker.com/target-policy: "true"` label in addition.

In DR clusters `Externalnamespaces` will be created after the clusters are coupled.

```sh
$ kubectl -n network-policy-operator get extns
NAME                                                                            AGE
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-0c83b447-0319-47a0-8c31-7c839a23adb6   43h
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-125cd336-dd11-4d38-8a12-6765e2509da6   43h
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-226fe157-b54a-496c-a23e-fe4f10b17e42   43h
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-228b16c3-d169-4b18-96d1-42e81a2dfc2c   43h
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-23bbcb70-92c6-4974-acb4-fffdeaf762db   43h
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-37c4fce2-c0aa-493e-9f0b-5e3566f2fb0b   43h
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-5a66930b-4ddd-4e87-b523-c685848790d2   43h
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-5dd0c278-6f59-455b-a764-dcfa2846df0f   43h
```

Due to long names, it is easier to get the particular `Externalnamespace` by label that describes the original name of namespace:

```sh
$ kubectl -n network-policy-operator get extns -l kubernetes.io/metadata.name=postgres-test
NAME                                                                            AGE
ens-28f4567e-c009-4549-8c83-a0d3d3fdc5bf-0c83b447-0319-47a0-8c31-7c839a23adb6   43h
```

## Audit Logs

The `NetworkPolicyConfig` allows to manage the connections logs collecting. The following NetworkPolicyConfig options allows to configure logs.

```yaml
spec:
  logs:
    enabled: false
    collectingAll: true
    namespaceScope: "network-policy.netcracker.com/collecting-logs"
```

* `logs.enable` enables and disables the collecting connection logs
* `logs.collectingAll` allowes to change collecting logs for all of the connections or only denied connections
* `logs.namespaceScope` sets the namespace label key that could be used to exclude some namespaces from collecting logs flexibly, if the values of the label is `none` or `false` connections are not logged in that namespace.

Connections logs for a particular pod are collected on the node, where the pod has been started. If the logs collecting has been enabled, it is possible to revise them on the following manner:

1. Connect to the particular node by SSH or by usage the `kubectl debug` option.
2. Run the `journalctl` command, for example:

```sh
# journalctl --all --since "2024-12-20 0:0:00" | grep 'calico-packet'
Dec 20 08:06:00 k8s-1-master-1 kernel: calico-packet: IN=tunl0 OUT=cali5da413c9ae4 MAC= SRC=10.128.227.48 DST=10.131.25.196 LEN=60 TOS=0x00 PREC=0x00 TTL=62 ID=25341 DF PROTO=TCP SPT=49884 DPT=24220 WINDOW=65330 RES=0x00 SYN URGP=0
Dec 20 08:06:01 k8s-1-master-1 kernel: calico-packet: IN=tunl0 OUT=cali5da413c9ae4 MAC= SRC=10.128.227.48 DST=10.131.25.196 LEN=60 TOS=0x00 PREC=0x00 TTL=62 ID=25342 DF PROTO=TCP SPT=49884 DPT=24220 WINDOW=65330 RES=0x00 SYN URGP=0
Dec 20 08:06:03 k8s-1-master-1 kernel: calico-packet: IN=tunl0 OUT=cali5da413c9ae4 MAC= SRC=10.128.227.48 DST=10.131.25.196 LEN=60 TOS=0x00 PREC=0x00 TTL=62 ID=25343 DF PROTO=TCP SPT=49884 DPT=24220 WINDOW=65330 RES=0x00 SYN URGP=0
```

## Upgrade with Helm Chart

Check the [Prerequisites](https://github.com/Netcracker/qubership-network-policy-operator/tree/main/documentation/Installation.md#prerequisites) from the _Network Policy Operator Installation Procedure_. Navigate to the `chart/network-policy-operator/` directory and edit `values.yaml` according to environment configuration. Upgrade CRDs:

```sh
kubectl apply -f crds/
```

Run upgrade procedure:

```sh
helm upgrade npc-operator . -f values.yaml -n network-policy-operator
```

**Notice:** `RemoteClusterConfig` resource is being recreated during the upgrade procedure it causes the `ExternalNamespaces` deletion on both sides
