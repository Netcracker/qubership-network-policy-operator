# Troubleshoot Qubership Network Policy Operator

APM package providing the public troubleshooting skill for Qubership Network Policy Operator.

The package covers operator reconciliation, Calico NetworkPolicy generation, DR cluster coupling,
and the documented TLS authentication failure. Its reference copy mirrors
[`docs/public/Troubleshooting.md`](../../docs/public/Troubleshooting.md).

## Structure

```text
agent-packages/troubleshoot-qubership-network-policy-operator/
├── apm.yml
└── .apm/
    ├── instructions/
    │   └── troubleshoot-qubership-network-policy-operator.instructions.md
    └── skills/
        └── troubleshoot-qubership-network-policy-operator/
            ├── SKILL.md
            └── references/
                └── Troubleshooting.md
```

The GitHub documentation-sync workflow keeps the reference copy aligned with the canonical guide.
Netcracker-specific delivery troubleshooting belongs in the companion package maintained in the
internal `network-policy-operator` repository.
