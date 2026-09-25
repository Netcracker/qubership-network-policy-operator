---
name: troubleshoot-qubership-network-policy-operator
description: >-
  Diagnose Qubership Network Policy Operator deployment and reconciliation failures involving
  NetworkPolicyConfig, RemoteClusterConfig, ExternalNamespace, Calico NetworkPolicies, namespace
  traffic rules, DR cluster coupling, or remote-cluster TLS and token authentication.
---

# Troubleshoot Qubership Network Policy Operator

## Deployment model

Network Policy Operator is delivered through two Helm charts:

| Chart | Purpose |
| --- | --- |
| `network-policy-operator` | Operator deployment, RBAC, CRDs, and monitoring resources |
| `network-policy-operator-cr` | `NetworkPolicyConfig`, `RemoteClusterConfig`, DR credentials, and related resources |

Install the operator chart before the CR chart so the CRDs exist before custom resources are
created. In DR deployments, each cluster has a `RemoteClusterConfig` for its peer and credentials
for the remote Kubernetes API.

## Evidence sources

| Artifact | Assessment purpose |
| --- | --- |
| Helm or ArgoCD deployment log | chart order, CRD, RBAC, and admission failures |
| Operator pod log | reconciliation, remote API, authentication, and policy-generation errors |
| `NetworkPolicyConfig` manifest and status | effective scope, rules, enabled flag, and reconciliation conditions |
| `RemoteClusterConfig` manifests and status | peer API, secret reference, authentication type, and coupling state |
| `ExternalNamespace` objects | whether remote namespace and endpoint data reached the local cluster |
| RoleBindings in the operator namespace | whether token or TLS identities have the required permissions |
| Secret metadata only | credential name/type consistency; never request or expose secret payloads |
| Calico `NetworkPolicy`, `Tier`, and `IPPool` resources | generated policy state, tier availability, and IPIP configuration |

If the required artifacts are absent or insufficient, recommend only the evidence needed for the
reported symptom, and stop without selecting a cause.

## Assessment flow

1. Confirm the issue belongs to Network Policy Operator. Look for its controller names,
   `NetworkPolicyConfig`, `RemoteClusterConfig`, `ExternalNamespace`, or
   `network-policy.netcracker.com` identifiers.
2. Separate deployment failures from runtime reconciliation failures.
3. For deployment failures, verify operator-chart-before-CR-chart ordering and inspect the first
   failed Kubernetes resource.
4. For local policy failures, correlate `NetworkPolicyConfig` scope and rules with generated Calico
   policies and operator logs.
5. For DR failures, correlate both clusters by peer name, remote API, authentication type, secret
   name, RoleBinding subject, and `RemoteClusterConfig.status.conditions`.
6. Match known signatures below. For a match, read only the corresponding reference section and
   apply it to the collected evidence.

To locate reference sections without loading the whole file:

```sh
grep -n "^## " references/Troubleshooting.md
```

## Configuration invariants

- `networkPolicies.enabled: false` disables generated policies globally.
- A non-empty `networkPolicies.scope` limits processing to namespaces with matching labels.
- `namespacesRules` takes precedence over the deprecated allow-annotation mechanism.
- `controlPlane` excludes matching namespace prefixes from restriction.
- `implicitNamespaces` grants the configured infrastructure namespaces broad access.
- DR peer names must be unique and the remote API and referenced credential must describe the same
  peer cluster.
- TLS-generated identities use the `<secret-name>-tls` username convention.
- Operator-managed Calico policies carry `network-policy.netcracker.com/generated: "true"`.

## Failure modes

### DR TLS authentication forbidden

**Signature:** operator logs report that a user such as `<secret-name>-tls` cannot list
`remoteclusterconfigs.netcracker.com`, usually from
`RemoteClusterConfigReconciler.GetCouple`, followed by `identification error`.
→ see `references/Troubleshooting.md` section **DR Authentication Errors**.

## Forbidden actions

- Do not expose Secret `.data` or decoded credentials in an assessment.
- Do not delete CRDs; that removes component resources cluster-wide.
- Do not edit or delete operator-managed Calico policies as a durable fix; reconciliation restores
  desired state.
- Do not force-remove a `RemoteClusterConfig` finalizer until the effect on remote
  `ExternalNamespace` cleanup is understood and explicitly accepted.
- Do not recommend disabling policies globally merely to bypass one traffic problem.

## Recommendation discipline

- Tie every proposed cause to an observed log, status condition, or configuration value.
- Prefer chart/value corrections followed by normal redeployment over direct mutation of managed
  resources.
- State the affected cluster when describing a DR-side change.
- Include a read-only verification step and the expected recovery signal: successful reconciliation,
  `Coupled=True`, updated `ExternalNamespace` objects, or corrected generated policies.
- If the evidence supports multiple causes, list the checks that distinguish them instead of
  selecting one without proof.
