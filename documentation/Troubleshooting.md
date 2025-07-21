<!-- #GFCFilterMarkerStart# -->
# Network Policy Operator Troubleshooting Guide
<!-- #GFCFilterMarkerEnd# -->

- [DR Authentication Errors](#dr-authentication-errors)

## DR Authentication Errors

### Description
The DR interaction doesn't work due to the authentication error e.g.:
```
2025-07-14T15:41:26Z ERROR Failed to get list of RemoteClusterConfigs {"error": "remoteclusterconfigs.qubership.org is forbidden: User \"cluster-2-secret-tls\" cannot list resource \"remoteclusterconfigs\" in API group \"qubership.org\" in the namespace \"network-policy-operator\""}
github.com/Netcracker/qubership-network-policy-operator/internal/controllers/rclc.(*RemoteClusterConfigReconciler).GetCouple
/workspace/internal/controllers/rclc/remoteclusterconfig_controller.go:397
github.com/Netcracker/qubership-network-policy-operator/internal/controllers/rclc.(*RemoteClusterConfigReconciler).Reconcile
/workspace/internal/controllers/rclc/remoteclusterconfig_controller.go:180
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).Reconcile
/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:114
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler
/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:311
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem
/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:261
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).Start.func2.2
/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:222
2025-07-14T15:41:26Z INFO Failed to get couple for 'remoteclusterconfig-ocp-cert-2' RemoteClusterConfig: remoteclusterconfigs.qubership.org is forbidden: User "cluster-2-secret-tls" cannot list resource "remoteclusterconfigs" in API group "qubership.org" in the namespace "network-policy-operator" {"controller": "remoteclusterconfig", "controllerGroup": "qubership.org", "controllerKind": "RemoteClusterConfig", "RemoteClusterConfig": {"name":"remoteclusterconfig-ocp-cert-2","namespace":"network-policy-operator"}, "namespace": "network-policy-operator", "name": "remoteclusterconfig-ocp-cert-2", "reconcileID": "60b39a34-1b3e-4c68-ab95-a330e3ddcca5"}
2025-07-14T15:41:26Z INFO RemoteClusterConfig Reconciler loop has been finished {"controller": "remoteclusterconfig", "controllerGroup": "qubership.org", "controllerKind": "RemoteClusterConfig", "RemoteClusterConfig": {"name":"remoteclusterconfig-ocp-cert-2","namespace":"network-policy-operator"}, "namespace": "network-policy-operator", "name": "remoteclusterconfig-ocp-cert-2", "reconcileID": "60b39a34-1b3e-4c68-ab95-a330e3ddcca5"}
2025-07-14T15:41:26Z ERROR Reconciler error {"controller": "remoteclusterconfig", "controllerGroup": "qubership.org", "controllerKind": "RemoteClusterConfig", "RemoteClusterConfig": {"name":"remoteclusterconfig-ocp-cert-2","namespace":"network-policy-operator"}, "namespace": "network-policy-operator", "name": "remoteclusterconfig-ocp-cert-2", "reconcileID": "60b39a34-1b3e-4c68-ab95-a330e3ddcca5", "error": "identification error"}
```

### How to resolve
1. Check [DR Preparation](https://github.com/Netcracker/qubership-network-policy-operator/blob/main/documentation/Installation.md#dr-preparation) has been fulfilled.
2. In case of TLS authentication usage it could be known issue. The fix is to add `-tls` suffix in the DR `RoleBinding` in Network Policy Operator namespace, e.g.:

The original notation:
```
...
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: cluster-2-secret
```
The valid notation:
```
...
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: cluster-2-secret-tls
```
