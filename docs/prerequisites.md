# Prerequisites

Setec runs workloads inside [Firecracker](https://firecracker-microvm.github.io/)
microVMs via [Kata Containers](https://katacontainers.io/). The operator
itself has modest requirements — but the Nodes that run `Sandbox` workloads
must be able to create real virtual machines. This document explains what
that means and how to prepare a cluster.

## Why microVM isolation needs KVM

Firecracker is a [Kernel-based Virtual Machine](https://www.linux-kvm.org/)
(KVM) monitor. It boots a guest kernel inside a hardware-virtualized
context provided by the host's CPU and the Linux KVM subsystem. That
hardware boundary is what makes microVM isolation stronger than shared-
kernel container isolation: a workload that escapes its namespace still
faces a full guest kernel and a virtualization boundary before it reaches
the host. Without KVM (`/dev/kvm`), Firecracker cannot start a VM, Kata
cannot schedule a Kata-runtime Pod, and Setec has no way to run your
`Sandbox`. There is no software-emulated fallback in this path.

## Why bare-metal or nested virtualization is required

A Node needs direct or pass-through access to the CPU's virtualization
extensions (Intel VT-x / AMD-V) exposed through `/dev/kvm`. In practice
that means one of the following:

- A **bare-metal Linux host** — virtualization extensions are available
  natively and KVM works out of the box (given an appropriate kernel).
- A **VM with nested virtualization enabled** — the outer hypervisor must
  be configured to pass VT-x/AMD-V into the guest. Nested virt carries a
  performance cost and configuration varies by host hypervisor; consult
  your hypervisor's documentation. If the guest does not see `/dev/kvm`,
  nested virt is not enabled.

You can verify KVM availability on a candidate Node:

```bash
# On the Node itself (e.g., via SSH or a debug Pod):
ls -l /dev/kvm
kvm-ok   # from the cpu-checker package on Debian/Ubuntu-like distros
```

Setec does not detect, depend on, or favor any cloud or vendor. Any
conformant Kubernetes distribution whose Nodes expose `/dev/kvm` will
work.

## Installing Kata Containers

Installing Kata is out of Setec's scope. Use the upstream project; it is
the authoritative source and is vendor-neutral.

- Project home: <https://katacontainers.io/>
- Installation docs: <https://github.com/kata-containers/kata-containers/blob/main/docs/install/README.md>
- `kata-deploy` (manifest-based installer):
  <https://github.com/kata-containers/kata-containers/tree/main/tools/packaging/kata-deploy>

The quickest path on a prepared cluster is `kata-deploy`, which ships as
a DaemonSet that lays down Kata binaries on every labeled Node and
registers the Kata `RuntimeClass` objects. Setec specifically needs the
`kata-fc` RuntimeClass (Firecracker VMM); `kata-deploy` creates it by
default. If your environment uses a non-default RuntimeClass name, set
`runtimeClassName` in `values.yaml` when installing the Setec chart.

## Node labeling

Setec's startup prerequisite check reads a Node label to identify which
Nodes are Kata-capable. The default label key is:

```
katacontainers.io/kata-runtime
```

`kata-deploy` applies this label to Nodes it has installed on, so in most
installations nothing extra is needed. If you install Kata by hand, label
your Kata-capable Nodes yourself:

```bash
kubectl label node <node-name> katacontainers.io/kata-runtime=true
```

You can override the label key the operator checks by passing
`nodeSelectorLabel` in the Helm chart values.

This label is informational for the operator's readiness signal — actual
scheduling is driven by the `kata-fc` RuntimeClass, which carries its own
node selectors from `kata-deploy`.

## Representative consumer scenarios

Setec is a substrate. These are illustrative workload patterns — not
endorsements of any specific downstream product.

- **AI agent code execution.** An agent system generates code on the fly
  and needs to execute it against real interpreters (Python, shell, etc.)
  without granting that code access to the host, the agent's runtime, or
  other tenants' data.
- **CI and build sandboxing.** Per-job microVMs run untrusted build
  scripts, `Dockerfile` instructions, or post-install hooks from third-
  party packages with a hardware isolation boundary between jobs.
- **Security research.** Malware triage, detonation of suspicious
  samples, or fuzzing harnesses run inside short-lived microVMs that are
  discarded after each run.
- **Ephemeral developer environments.** A platform provisions a fresh
  microVM per pull-request preview or per interactive session, isolating
  the user's environment from every other user's and from the platform's
  control plane.

In all four cases the interface is the same: apply a `Sandbox` CR, read
the phase and logs, delete it. Consumers talk to the CRD (or, in a future
phase, a gRPC frontend); Setec is unaware of and undifferentiated by who
its consumers are.
