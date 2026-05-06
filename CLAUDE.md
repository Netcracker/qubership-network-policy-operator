# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Does

QuberShip Network Policy Operator is a Kubernetes operator that manages Calico `NetworkPolicy` resources in OCP/Kubernetes clusters using custom resources and namespace annotations. It supports multi-cluster (DR) scenarios via `RemoteClusterConfig`.

## Commands

```sh
# Build
make build            # compiles to bin/manager (also runs manifests, generate, fmt, vet)
make run              # run controller locally against current kubeconfig

# Test
make test             # unit tests with envtest (K8s 1.28.3), excludes e2e, produces cover.out
go test ./internal/controllers/npc/... -v  # run a single controller's tests

# Lint
make lint             # golangci-lint v1.54.2
make lint-fix         # golangci-lint with auto-fix

# Code generation (required after changing API types or RBAC markers)
make generate         # regenerates zz_generated.deepcopy.go
make manifests        # regenerates CRD/RBAC YAMLs in config/crd/bases and config/rbac

# Deploy
make install          # install CRDs into cluster
make deploy IMG=<img> # deploy operator to cluster
```

## Architecture

The operator uses [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) and follows the standard kubebuilder v4 layout.

### Controllers (`internal/controllers/`)

Four reconcilers, each in its own package:

| Package | Reconciler | Watches |
|---------|-----------|---------|
| `npc/` | `NetworkPolicyConfigReconciler` | `NetworkPolicyConfig` (cluster-scoped CR) — the central config that drives policy generation across namespaces |
| `ns/` | `NamespaceReconciler` | `Namespace` — triggers policy reconciliation when namespace labels/annotations change |
| `cnp/` | `NetworkPolicyReconciler` | Calico `NetworkPolicy` (projectcalico.org/v1) — ensures generated policies stay in sync |
| `rclc/` | `RemoteClusterConfigReconciler` | `RemoteClusterConfig` — manages cross-cluster connectivity for DR scenarios, syncing `ExternalNamespace` objects |

The `npc` controller is the primary orchestrator. It reads `NetworkPolicyConfig`, iterates over namespaces, and creates/updates/deletes Calico `NetworkPolicy` objects. The `ns` and `cnp` controllers enqueue `NetworkPolicyConfig` reconciliations when owned resources change.

### Custom Resources (`api/v1alpha1/`)

- **`NetworkPolicyConfig`** (cluster-scoped) — main configuration: `enabled` flag, `scope` (namespace label selector), `policies` map, annotation names, Calico tier names, and logging settings.
- **`RemoteClusterConfig`** (cluster-scoped) — defines a remote cluster connection: API URL, auth secret reference, and tunnel address annotation.
- **`ExternalNamespace`** — synced from remote clusters by the `rclc` controller; represents a remote namespace with its pod/service IP data so the `npc` controller can generate egress rules to reach them.

### Key Labels / Annotations

- `network-policy.netcracker.com/generated` — label applied to all operator-managed Calico NetworkPolicies.
- `network-policy.netcracker.com/egress-svc` — label for egress service policies.
- `NetworkPolicyConfig.spec.allowAnnotation` — namespace annotation that opts a namespace into custom policies.

### Metrics (`monitoring/`)

Prometheus metrics are registered at startup via `monitoring.RegisterMetrics()` and exposed on `:8080`.

### Generated Files

`api/v1alpha1/zz_generated.deepcopy.go` and files under `config/crd/bases/` and `config/rbac/` are auto-generated — do not edit manually. Regenerate with `make generate` and `make manifests` after changing API types or kubebuilder markers.

## Prerequisites

- Go 1.22+
- Calico CNI with API server v3.28.1+
- Kubernetes v1.27+ or OCP 4.14+
