# Development

## Prerequisites
- go version v1.22.0+
- docker version 17.03+
- kubectl version v1.11.3+
- Calico CNI with API server version v3.28.1+
- Kubernetes v1.27+ or OCP4.14+ cluster
- In case of DR, the following scheme must be configured: [DR Scheme](https://github.com/Netcracker/DRNavigator/tree/main/documentation)

## To Deploy on the cluster

**Deploy the operator to the cluster:**

```sh
make deploy
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Deploy default NetworkPolicyConfig:**

```sh
kubectl apply -k config/samples/networkpolicyconfig.yaml
```

> **NOTE**: The file above could be changed if it's needed

**Prepare DR on both clusters:**

```sh
make set-dr1
make set-dr2
```

**Prepare DR on Cluster1:**

Run the command

```sh
cat token-dr2
```

than:

```sh
kubectl -n netpolconf-system patch secret remote-cluster2  --type json -p '[{"op": "replace", "path": "/data/token", "value": "<PREVIOUS COMMAND OUTPUT>"}]'
```

Edit config/samples/remoteclusterconfig_1.yaml and apply it:

```sh
kubectl -n netpolconf-system apply -k config/samples/remoteclusterconfig_1.yaml
```

**Prepare DR on Cluster2:**

Run the command

```sh
cat token-dr1
```

than:

```sh
kubectl -n netpolconf-system patch secret remote-cluster1  --type json -p '[{"op": "replace", "path": "/data/token", "value": "<PREVIOUS COMMAND OUTPUT>"}]'
```

Edit config/samples/remoteclusterconfig_2.yaml and apply it:

```sh
kubectl -n netpolconf-system apply -k config/samples/remoteclusterconfig_2.yaml
```

## To Uninstall

**Remove DR on both clusters:**

```sh
make -i unset-dr1
make -i unset-dr2
```

**UnDeploy the operator from the cluster:**

```sh
make -i undeploy
```

## Contributing

> **NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
